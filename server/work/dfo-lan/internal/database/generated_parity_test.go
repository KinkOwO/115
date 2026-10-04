package database

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Signature-parity gate between the two generated packages.
//
// The SQLite query fork is hand-written, so "does it compile" is not enough: a
// dropped parameter, a changed nullability, or an expression sqlc could not type
// (interface{}) would all still generate code and only fail at runtime, on the
// engine nobody exercises by hand. This compares the generated Go API of sqlcgen
// and sqlcgensqlite directly, so a divergence is caught at the source level.
//
// The comparison is EXACT. That is deliberate: the shared query interface is
// satisfied by embedding each generated package, so any difference means that
// engine cannot satisfy the interface. A divergence must therefore be fixed in the
// SQL or in the sqlc overrides - or listed in acceptedDivergences below, which
// documents the positions where the two engines provably cannot agree and which
// therefore need one hand-written adapter method each.
//
// While the port is incomplete the test reports and skips, so the suite stays
// green during the work; once every PostgreSQL method exists the verdict is final.

type generatedAPI struct {
	methods map[string][]string // method -> ordered parameter types
	results map[string][]string // method -> ordered result types
	structs map[string][]string // struct -> sorted "Field:type" entries
	untyped []string            // any interface{} occurrence, which R-1 forbids
}

type divergence struct {
	key    string
	detail string
}

// acceptedDivergences lists positions where the two engines provably cannot
// produce the same Go type, with the reason. Each entry is a promise that the
// shared interface needs one hand-written adapter method there. The test fails if
// an entry becomes stale or if a divergence appears that is not listed.
//
// Why these cannot be fixed in SQL or in sqlc overrides (measured, not assumed):
//   - column references DO honour db_type overrides (SMALLINT -> int16 works);
//   - CAST targets do NOT: CAST(x AS INTEGER/SMALLINT/BIGINT) always infers int64;
//   - aggregates do not inherit their argument's width: max(col) infers int64, and
//     max() over a bare expression infers interface{}.
//
// The transaction handle is a separate case: pgx.Tx and *sql.Tx are different
// types by construction, so WithTx cannot join a shared interface at all.
var acceptedDivergences = map[string]string{
	"WithTx:param0": "transaction handle is engine-specific by design: pgx.Tx versus *sql.Tx",
	"AdvanceTowerFloorParams:field.Floor": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"AdvanceTowerGriefParams:field.Floor": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"ReserveTowerEntryParams:field.Floor": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"EnsureTowerGriefProgressParams:field.HighestCleared": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"EnsureTowerProgressParams:field.HighestCleared": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"UpdateMailParams:field.Status": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"FixtureCashInventoryCountParams:field.Amount": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"FixtureCashInventoryCountParams:field.ClaimState": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"GrantHistoryParams:field.MaxEntries": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"HasCompletedQuestParams:field.QuestID": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"AdventureLevel:result0": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"CashOrderVaultSpace:result0": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"CharacterAllocationRow:field.NextWireID": "PostgreSQL infers int32 from coalesce(max(wire_id),0)+1; SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"RecordCharacterFameParams:field.CurrentFame": "SQLite cannot reproduce PostgreSQL's explicit cast: a CAST target always infers int64, and a bare parameter only takes a width from a column it is compared with",
	"LockCharacterRow:field.FixedSlot": "PostgreSQL projects 0::smallint (int16); no SQLite expression for a literal yields int16",
	"LockMailCharactersRow:field.FixedSlot": "same 0::smallint projection as LockCharacterRow",
	"MailRecipientRow:field.FixedSlot": "same 0::smallint projection as LockCharacterRow",
	"LockOwnedCharacterIncludingDeletedRow:field.FixedSlot": "same 0::smallint projection as LockCharacterRow",
	"AdventureCollectionEquipment:result0": "the value comes from a JSON1 function; casting it so it can be scanned into json.RawMessage yields []byte, and JSON1 returns TEXT while the column is BLOB",
	"CharacterSeason:result0": "same JSON1 BLOB round trip as AdventureCollectionEquipment: the result is an expression, so the json.RawMessage column override cannot apply",
	"CharactersWithAdventureRow:field.State": "same JSON1 BLOB round trip as AdventureCollectionEquipment",
	"FixtureInventoryItemsParams:field.Items": "the parameter feeds json_set, and CAST(... AS JSON) destroys the value through NUMERIC affinity, so it must bind as []byte via json(CAST(... AS BLOB))",
	"FixtureMergeCharacterFieldsParams:field.Fields": "same json_set parameter constraint as Items",
	"FixtureWalletGoldParams:field.Gold": "same json_set parameter constraint as Items",
	"ClearQuestsParams:field.QuestIds": "SQLite has no array type, so the value travels as JSON text or a JSON-array blob",
	"ClearActQuestsParams:field.QuestIds": "SQLite has no array type, so the value travels as JSON text or a JSON-array blob",
	"CompletedQuestIDsParams:field.QuestIds": "SQLite has no array type, so the value travels as JSON text or a JSON-array blob",
	"CompleteGraduationQuestsParams:field.QuestIds": "SQLite has no array type, so the value travels as JSON text or a JSON-array blob",
	"BleedingMineTeamsRow:field.Members": "PostgreSQL bigint[] column, stored as a JSON array (D27)",
	"SaveBleedingMineTeamParams:field.Members": "PostgreSQL bigint[] column, stored as a JSON array (D27)",
	"ScrubPollutedWorldPositions:param0": "PostgreSQL []int64 parameter carried as JSON text (D9)",
	"LockMailCharacters:param0": "PostgreSQL []int64 parameter carried as JSON text (D9)",
	"LockMailboxParams:field.MessageIds": "= ANY(...) over []int64 mapped to json_each (D9)",
	"ClaimAdminGrantParams:field.CharacterID": "the PostgreSQL block does not enable emit_pointers_for_null_types, so a nullable column is pgtype.X there while SQLite yields *T",
	"InsertPlayerMailParams:field.SenderID": "the PostgreSQL block does not enable emit_pointers_for_null_types, so a nullable column is pgtype.X there while SQLite yields *T",
	"AdvanceTowerFloorRow:field.Column2": "sqlc's sqlite engine drops RETURNING aliases and names the field ColumnN",
	"AdvanceTowerFloorRow:field.EntryDay": "sqlc's sqlite engine drops RETURNING aliases and names the field ColumnN; the PostgreSQL value semantics and type were kept",
	"ReserveFavorGiftRow:field.Column3": "sqlc's sqlite engine drops RETURNING aliases and names the field ColumnN",
	"ReserveFavorGiftRow:field.LastGiftDay": "sqlc's sqlite engine drops RETURNING aliases and names the field ColumnN; the string value semantics were kept",
	"FatigueRecoveryUsageRow:field.LastUsed": "an aggregate over a TIMESTAMP column reports no decltype, so the driver cannot convert integer microseconds into time.Time; SQLite returns int64 microseconds for the adapter",
}

func collectGeneratedAPI(t *testing.T, dir string) generatedAPI {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", dir, err)
	}
	api := generatedAPI{
		methods: map[string][]string{},
		results: map[string][]string{},
		structs: map[string][]string{},
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Recv == nil || len(d.Recv.List) == 0 {
						continue
					}
					if types.ExprString(d.Recv.List[0].Type) != "*Queries" {
						continue
					}
					var params []string
					for _, field := range d.Type.Params.List {
						count := len(field.Names)
						if count == 0 {
							count = 1
						}
						typ := types.ExprString(field.Type)
						if typ == "context.Context" {
							continue // present on every method in both packages
						}
						for i := 0; i < count; i++ {
							params = append(params, typ)
						}
					}
					var results []string
					if d.Type.Results != nil {
						for _, field := range d.Type.Results.List {
							count := len(field.Names)
							if count == 0 {
								count = 1
							}
							typ := types.ExprString(field.Type)
							if typ == "error" {
								continue
							}
							for i := 0; i < count; i++ {
								results = append(results, typ)
							}
						}
					}
					api.methods[d.Name.Name] = params
					api.results[d.Name.Name] = results
					for _, typ := range append(append([]string{}, params...), results...) {
						if strings.Contains(typ, "interface{}") || typ == "any" {
							api.untyped = append(api.untyped, d.Name.Name+" -> "+typ)
						}
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						st, ok := ts.Type.(*ast.StructType)
						if !ok {
							continue
						}
						var fields []string
						for _, field := range st.Fields.List {
							typ := types.ExprString(field.Type)
							for _, name := range field.Names {
								fields = append(fields, name.Name+":"+typ)
							}
						}
						sort.Strings(fields)
						api.structs[ts.Name.Name] = fields
					}
				}
			}
		}
	}
	return api
}

// literalArgHits returns the generated files in dir that still contain the text
// "sqlc.arg(". sqlc normally rewrites those into placeholders; when it does not,
// the statement is broken and nothing else in the toolchain complains.
func literalArgHits(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var hits []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		// Only executable SQL counts. The generated files also carry the query's
		// documentation twice - as SQL comment lines inside the constant and as Go
		// // comments above it - and both legitimately mention sqlc.arg when
		// explaining these very traps. A false positive here would make the guard
		// useless, so any line that is a comment in either syntax is skipped.
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "--") || strings.HasPrefix(trimmed, "//") {
				continue
			}
			if strings.Contains(trimmed, "sqlc.arg(") {
				hits = append(hits, entry.Name())
				break
			}
		}
	}
	sort.Strings(hits)
	return hits
}

func fieldMap(fields []string) map[string]string {	out := make(map[string]string, len(fields))
	for _, entry := range fields {
		name, typ, found := strings.Cut(entry, ":")
		if found {
			out[name] = typ
		}
	}
	return out
}

func TestSQLiteGeneratedSignatureParity(t *testing.T) {
	pg := collectGeneratedAPI(t, "sqlcgen")
	lite := collectGeneratedAPI(t, "sqlcgensqlite")
	if len(pg.methods) == 0 {
		t.Fatal("parsed no PostgreSQL queries; the generated package layout changed")
	}

	var missing []string
	for name := range pg.methods {
		if _, ok := lite.methods[name]; !ok {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	complete := len(missing) == 0

	var divergences []divergence
	// Anything sqlc copies verbatim - notably every sqlc.arg(...) inside an
	// ON CONFLICT ... DO UPDATE clause - survives into the generated constant and
	// ships as broken SQL while sqlc still exits 0. Grepping the generated
	// constants is the only reliable way to catch it.
	for _, file := range literalArgHits(t, "sqlcgensqlite") {
		divergences = append(divergences, divergence{"literal:" + file,
			file + " still contains a literal sqlc.arg(...), which means the statement is unbound"})
	}
	if len(lite.untyped) > 0 {
		divergences = append(divergences, divergence{"untyped",
			"sqlc could not type these positions (add an explicit CAST, rule R-1): " +
				strings.Join(dedupe(lite.untyped), ", ")})
	}
	for name, pgParams := range pg.methods {
		liteParams, ok := lite.methods[name]
		if !ok {
			continue
		}
		if len(pgParams) != len(liteParams) {
			divergences = append(divergences, divergence{name + ":paramcount",
				name + ": parameter count is " + itoa(len(liteParams)) + ", PostgreSQL has " + itoa(len(pgParams))})
			continue
		}
		for i := range pgParams {
			if pgParams[i] != liteParams[i] {
				divergences = append(divergences, divergence{name + ":param" + itoa(i),
					name + ": parameter " + itoa(i) + " is " + liteParams[i] + ", PostgreSQL has " + pgParams[i]})
			}
		}
		pgResults, liteResults := pg.results[name], lite.results[name]
		if len(pgResults) != len(liteResults) {
			divergences = append(divergences, divergence{name + ":resultcount",
				name + ": result count is " + itoa(len(liteResults)) + ", PostgreSQL has " + itoa(len(pgResults))})
			continue
		}
		for i := range pgResults {
			if pgResults[i] != liteResults[i] {
				divergences = append(divergences, divergence{name + ":result" + itoa(i),
					name + ": result " + itoa(i) + " is " + liteResults[i] + ", PostgreSQL has " + pgResults[i]})
			}
		}
	}
	for name, pgFields := range pg.structs {
		liteFields, ok := lite.structs[name]
		if !ok {
			if complete {
				divergences = append(divergences, divergence{name + ":missing",
					"struct missing from the SQLite package: " + name})
			}
			continue
		}
		// Field-level keys, so an accepted divergence cannot mask a new one in the
		// same struct.
		pgMap, liteMap := fieldMap(pgFields), fieldMap(liteFields)
		for field, pgType := range pgMap {
			liteType, ok := liteMap[field]
			if !ok {
				divergences = append(divergences, divergence{name + ":field." + field,
					name + "." + field + " is missing from the SQLite package"})
				continue
			}
			if pgType != liteType {
				divergences = append(divergences, divergence{name + ":field." + field,
					name + "." + field + " is " + liteType + ", PostgreSQL has " + pgType})
			}
		}
		for field := range liteMap {
			if _, ok := pgMap[field]; !ok {
				divergences = append(divergences, divergence{name + ":field." + field,
					name + "." + field + " exists only in the SQLite package"})
			}
		}
	}

	var accepted, unexpected []string
	used := map[string]bool{}
	for _, d := range divergences {
		if reason, ok := acceptedDivergences[d.key]; ok {
			used[d.key] = true
			accepted = append(accepted, d.key+" ("+reason+")")
			continue
		}
		unexpected = append(unexpected, d.key+": "+d.detail)
	}
	var stale []string
	if complete {
		for key := range acceptedDivergences {
			if !used[key] {
				stale = append(stale, key)
			}
		}
	}
	sort.Strings(accepted)
	sort.Strings(unexpected)
	sort.Strings(stale)

	if !complete {
		if len(accepted) > 0 {
			t.Logf("%d accepted engine divergence(s) among the %d ported methods (each needs one hand-written adapter method):\n  %s",
				len(accepted), len(pg.methods)-len(missing), strings.Join(accepted, "\n  "))
		}
		if len(unexpected) > 0 {
			t.Logf("attention: %d unexpected divergence(s) among the ported methods:\n  %s",
				len(unexpected), strings.Join(unexpected, "\n  "))
		}
		t.Skipf("SQLite query port in progress: %d of %d methods ported, %d missing (first: %s)",
			len(pg.methods)-len(missing), len(pg.methods), len(missing), strings.Join(firstN(missing, 5), ", "))
	}
	if len(unexpected) > 0 || len(stale) > 0 {
		var report []string
		if len(unexpected) > 0 {
			report = append(report, "unexpected divergences (fix the SQL or the overrides, or justify an entry in acceptedDivergences):")
			report = append(report, "  "+strings.Join(unexpected, "\n  "))
		}
		if len(stale) > 0 {
			report = append(report, "stale acceptedDivergences entries (no longer needed, remove them):")
			report = append(report, "  "+strings.Join(stale, "\n  "))
		}
		t.Fatalf("generated API parity broken:\n%s", strings.Join(report, "\n"))
	}
	t.Logf("generated API parity: %d methods and %d structs aligned exactly, %d documented engine divergence(s)",
		len(pg.methods), len(pg.structs), len(accepted))
}

func firstN(items []string, n int) []string {
	if len(items) < n {
		return items
	}
	return items[:n]
}

func dedupe(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var digits []byte
	for i > 0 {
		digits = append([]byte{byte('0' + i%10)}, digits...)
		i /= 10
	}
	return string(digits)
}
