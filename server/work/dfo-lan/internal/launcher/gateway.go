package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// This file is the gateway half of Stage 2: the argv channel_probe.py built
// (L227-L553 of server/work/dfo_probe_tools/channel_probe.py) plus the environment it
// injected (L486-L584), reproduced in the same order.
//
// 为什么要连"永远不会走到"的历史分支一起搬：命令行的每一个 token 都是与 Python 版逐参数
// 对比的验收对象，少搬一条分支就少一个可对比点，且 tag 是外部输入（--tag / 后续阶段），
// 分支是否可达不该由搬运者替它决定。

// sessionTag 是一次会话的 tag。目录用**原始** tag，行为按**降级后**的 tag —— 这正是
// channel_probe.py L20-L32 的行为：out 在降级前生成，后面的每个判断都看降级后的 tag。
// _next37 会一路降级到 _next34，所以 candidate35/36/37 同时为真。
type sessionTag struct {
	Original    string
	Effective   string
	Candidate35 bool
	Candidate36 bool
	Candidate37 bool
	Out         string
}

// newSessionTag 复刻 channel_probe.py L24-L32 的降级链。
func newSessionTag(project, tag string) sessionTag {
	session := sessionTag{Original: tag, Effective: tag, Out: filepath.Join(project, "runtime", tag)}
	// 逐级降级：_next37 -> _next36 -> _next35 -> _next34，每一步都设标志位。
	if strings.HasSuffix(session.Effective, "_next37") {
		session.Candidate37 = true
		session.Effective = strings.TrimSuffix(session.Effective, "_next37") + "_next36"
	}
	if strings.HasSuffix(session.Effective, "_next36") {
		session.Candidate36 = true
		session.Effective = strings.TrimSuffix(session.Effective, "_next36") + "_next35"
	}
	if strings.HasSuffix(session.Effective, "_next35") {
		session.Candidate35 = true
		session.Effective = strings.TrimSuffix(session.Effective, "_next35") + "_next34"
	}
	return session
}

// childEnv is the environment the gateway is started with. The base order is preserved
// (that is the order the launcher's own environment had) and Set either replaces a value in
// place or appends the variable, which is what os.environ did.
type childEnv struct {
	values map[string]string
	order  []string
	raw    []string
}

// newChildEnv 把 os.Environ() 形式的环境装进来。
func newChildEnv(base []string) *childEnv {
	env := &childEnv{values: map[string]string{}}
	for _, entry := range base {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			// 不是 KEY=VALUE 的条目原样保留，而不是丢掉（BuildServerEnv 也会这样带过去）。
			env.raw = append(env.raw, entry)
			continue
		}
		env.Set(key, value)
	}
	return env
}

// Get 对应 os.environ.get(key)：未设置与设置为空串都返回 ""，两者在 Python 的真值判断
// 里本来就等价。要区分"存在但为空"请用 Has。
func (e *childEnv) Get(key string) string { return e.values[key] }

// Has 对应 `key in os.environ`。
func (e *childEnv) Has(key string) bool {
	_, ok := e.values[key]
	return ok
}

// Set 写入一个变量：已存在则原位覆盖，否则追加。
func (e *childEnv) Set(key, value string) {
	if e.values == nil {
		e.values = map[string]string{}
	}
	if _, seen := e.values[key]; !seen {
		e.order = append(e.order, key)
	}
	e.values[key] = value
}

// List 渲染成 exec 需要的 KEY=VALUE 列表。
func (e *childEnv) List() []string {
	entries := make([]string, 0, len(e.order)+len(e.raw))
	for _, key := range e.order {
		entries = append(entries, key+"="+e.values[key])
	}
	return append(entries, e.raw...)
}

// gatewayCommandInput is everything the argv builder reads. Probe is injected so the
// builder itself stays free of process I/O: production passes ExeFlags, tests pass a
// table.
type gatewayCommandInput struct {
	Project string // server/work/dfo-lan
	Session sessionTag
	Env     *childEnv
	Probe   func(exe string) map[string]bool
}

// gatewayCommand 是构造结果：参数表加上 Python 会打出去的那些告警。两个切片分开，是因为
// Python 把它们送去了不同的流（prune/历史提示走 stdout，JSON 目录告警走 stderr），而这两个
// 流在 Python 版里分别是 helper.out / helper.err。
type gatewayCommand struct {
	Args     []string
	Notices  []string // channel_probe.py 的 print(...)
	Warnings []string // channel_probe.py 的 print(..., file=sys.stderr)
}

// buildGatewayCommand 复刻 channel_probe.py L227-L584 的下发命令行与环境构造。
func buildGatewayCommand(in gatewayCommandInput) (gatewayCommand, error) {
	var result gatewayCommand
	project := in.Project
	out := in.Session.Out
	tag := in.Session.Effective
	env := in.Env

	at := func(parts ...string) string { return filepath.Join(append([]string{project}, parts...)...) }
	persisted := strings.HasPrefix(tag, "roles_persist")

	binary := at("bin", "wireprobe.exe")
	if persisted {
		binary = at("bin", "wireprobe-character.exe")
	}
	command := []string{
		binary,
		"-fixture", filepath.Join(out, "channelinfo.bin"),
		"-output", out,
	}

	if persisted {
		// 带 odyssey_pilot 的规则优先，目录同理（channel_probe.py L235-L241）。
		characterRules := at("configs", "character-probe.json")
		if odyssey := at("configs", "character-rules.odyssey-release.json"); regularFile(odyssey) {
			characterRules = odyssey
		}
		characterCatalog := at("configs", "characters.generated.json")
		if skycastle := at("configs", "characters.skycastle-release.json"); regularFile(skycastle) {
			characterCatalog = skycastle
		}
		command = append(command,
			"-character-storage", at("runtime", "storage", "local.json"),
			"-character-catalog", characterCatalog,
			"-character-rules", characterRules,
		)
	}
	if strings.HasPrefix(tag, "roles_persist_select") {
		command = append(command, "-select-probe-config", at("cmd", "wireprobe", "testdata", "select-parser-probe.json"))
	}
	if strings.HasPrefix(tag, "roles_persist_select_actor") {
		command = append(command, "-entry-basic-probe")
	}
	if strings.HasPrefix(tag, "roles_persist_select_actor_town") {
		command = append(command,
			"-town-catalog", at("configs", "town.generated.json"),
			"-town-entry-probe", at("cmd", "wireprobe", "testdata", "town-entry-probe.json"),
		)
	}
	if tag != "channel_01" && tag != "channel_02" && tag != "channel_03" {
		command = append(command, "-responses", filepath.Join(out, "responses.json"))
	}
	if strings.HasPrefix(tag, "roles_persist_select_actor_town_world_live") {
		var err error
		if command, err = replaceFlagValue(command, "-select-probe-config",
			at("cmd", "wireprobe", "testdata", "select-world-probe.json")); err != nil {
			return result, err
		}
		command = append(command,
			"-world-rules", at("configs", "world-probe.json"),
			"-vault-rules", at("configs", "vault.generated.json"),
		)
		if strings.Contains(tag, "_detail_") {
			command = append(command,
				"-entry-addition-probe",
				"-fatigue-rules", at("configs", "fatigue-probe.json"),
			)
		}
	}
	if hasAnySuffix(tag, "_next26", "_next27", "_next28", "_next29", "_next30", "_next31", "_next32", "_next33", "_next34") {
		if !persisted || !strings.Contains(tag, "_dungeon_") {
			return result, fmt.Errorf("next26 需要完整的 dungeon 档（tag=%s）", tag)
		}
		command[0] = at("bin", "wireprobe-dungeon26.exe")
		// next25 代目录没有 advancement_growth / advancement_skills，建号选定的转职槽位
		// 无处落账；skycastle-release 是同一份 PVF 快照的补块导出，逐字段一致。
		var err error
		if command, err = replaceFlagValue(command, "-character-catalog",
			at("configs", "characters.skycastle-release.json")); err != nil {
			return result, err
		}
		command = append(command,
			"-progression-rules", at("configs", "experience.compat90.json"),
			"-loot-catalog", "pvf",
			"-loot-rules", at("configs", "drop.compat90.json"),
			"-bag-rules", at("configs", "inventory.compat90.json"),
			"-card-rules", at("configs", "cards.compat90.json"),
		)
		if hasAnySuffix(tag, "_next27", "_next28", "_next29", "_next30", "_next31", "_next32", "_next33", "_next34") {
			command[0] = at("bin", "wireprobe-dungeon27.exe")
		}
		if hasAnySuffix(tag, "_next28", "_next29", "_next30", "_next31", "_next32", "_next33", "_next34") {
			command[0] = at("bin", "wireprobe-dungeon28.exe")
			command = append(command, "-channel-refresh-config", at("configs", "channel.local28.json"))
		}
		if hasAnySuffix(tag, "_next29", "_next30", "_next31", "_next32", "_next33", "_next34") {
			command[0] = at("bin", "wireprobe-dungeon29.exe")
			var err error
			if command, err = replaceFlagValue(command, "-bag-rules", at("configs", "inventory.next29.json")); err != nil {
				return result, err
			}
			if !in.Session.Candidate35 && !in.Session.Candidate36 && !in.Session.Candidate37 {
				result.Notices = append(result.Notices, "NOTICE: next29-next34 are historical binaries. "+
					"Their retired quest-equipment JSON is no longer injected; use the matching historical "+
					"configuration, or build current source with native PVF.")
			}
		}
		if strings.HasSuffix(tag, "_next30") {
			command[0] = at("bin", "wireprobe-dungeon30.exe")
		}
		if strings.HasSuffix(tag, "_next31") {
			command[0] = at("bin", "wireprobe-dungeon31.exe")
			var err error
			if command, err = replaceFlagValue(command, "-channel-refresh-config", at("configs", "channel.local31.json")); err != nil {
				return result, err
			}
		}
		if strings.HasSuffix(tag, "_next32") {
			command[0] = at("bin", "wireprobe-dungeon32.exe")
			var err error
			if command, err = replaceFlagValue(command, "-channel-refresh-config", at("configs", "channel.local32.json")); err != nil {
				return result, err
			}
		}
		if strings.HasSuffix(tag, "_next33") {
			command[0] = at("bin", "wireprobe-dungeon33.exe")
			var err error
			if command, err = replaceFlagValue(command, "-channel-refresh-config", at("configs", "channel.local32.json")); err != nil {
				return result, err
			}
			command = append(command, "-game-listen", "127.0.0.2:0")
		}
		if strings.HasSuffix(tag, "_next34") {
			command[0] = at("bin", "wireprobe-dungeon33.exe")
			var err error
			if command, err = replaceFlagValue(command, "-channel-refresh-config", at("configs", "channel.local34.json")); err != nil {
				return result, err
			}
			command = append(command, "-game-listen", "127.0.0.2:0")
		}
	}
	if in.Session.Candidate35 {
		command[0] = at("bin", "wireprobe-dungeon35.exe")
		command = append(command,
			"-equipment-wear-rules", at("configs", "equipment-wear.current35.json"),
			"-account-options", at("configs", "account-options.current35.json"),
		)
	}
	if in.Session.Candidate36 {
		command[0] = at("bin", "wireprobe-dungeon36.exe")
		var err error
		if command, err = replaceFlagValue(command, "-loot-rules", at("configs", "drop.current36.json")); err != nil {
			return result, err
		}
		command = append(command,
			"-tutorial-routes", at("configs", "tutorial-routes.current35.json"),
			"-tutorial-dungeons", at("configs", "tutorial-dungeons.current36.json"),
			"-solo-party-bootstrap",
		)
	}
	if in.Session.Candidate37 {
		command[0] = at("bin", "wireprobe-dungeon39.exe")
		// next37 用扩展后的 50 行频道目录，并把快捷栏槽位补进背包策略。
		var err error
		if command, err = replaceFlagValue(command, "-channel-refresh-config", at("configs", "channel.local35.json")); err != nil {
			return result, err
		}
		if command, err = replaceFlagValue(command, "-bag-rules", at("configs", "inventory.current37.json")); err != nil {
			return result, err
		}
		if itemShop := at("configs", "itemshop-candidate.json"); regularFile(itemShop) {
			command = append(command, "-item-shop", itemShop)
		}
		// 出厂相对默认值在"网关 cwd 是包根"的启动路径下永远解析不到，所以给绝对路径；
		// 故意不放在 exists() 后面：文件缺失时服务端会打印它试过的绝对路径。
		command = append(command, "-apocalypse-catalog", at("configs", "apocalypse.generated.json"))
		if randomOption := at("configs", "randomoption.current37.json"); regularFile(randomOption) {
			command = append(command, "-random-option-catalog", randomOption)
		}
	}
	// 维纳斯终局翻牌装备池：与 apocalypse 同理必须给绝对路径 —— 网关的 cwd 不是
	// dfo-lan，出厂相对默认值 configs/venus-flip-gear.generated.json 解析不到，
	// Scenario Mode 会因找不到配置文件启动失败。旧二进制没有这个 flag 时由
	// PruneCommand 按 exe -h 的能力探测自动丢弃（连同它的值）。
	command = append(command, "-venus-flip-gear", at("configs", "venus-flip-gear.generated.json"))
	// 服务端程序由外层选定（本启动器写 DFO_SERVER_BINARY）。
	if chosen := env.Get("DFO_SERVER_BINARY"); chosen != "" {
		command[0] = chosen
	}
	if env.Get("DFO_CHANNEL_IDENTITY") == "1" {
		command = append(command, "-channel-identity")
	}
	// 三个角色档案开关允许用环境变量覆盖命令行（channel_probe.py L458-L464）。
	for _, option := range []struct{ flag, key string }{
		{"-character-storage", "DFO_CHARACTER_STORAGE"},
		{"-character-catalog", "DFO_CHARACTER_CATALOG"},
		{"-character-rules", "DFO_CHARACTER_RULES"},
	} {
		if env.Has(option.key) {
			var err error
			if command, err = replaceFlagValue(command, option.flag, env.Get(option.key)); err != nil {
				return result, err
			}
		}
	}
	// 覆盖登录应答：Python 在这里直接改 responses.json。
	if responses := commandFlagValue(command, "-responses"); responses != "" {
		OverrideLoginResponse(responses, project, env.Get("DFO_LOGIN_RESPONSE"))
	}
	warnings, err := validateJSONCatalogs(command, env)
	if err != nil {
		return result, err
	}
	result.Warnings = append(result.Warnings, warnings...)

	// 奥德赛组件常驻挂载（不再看 DFO_ODYSSEY_MODE）：少了这些，奥德赛角色进城后没有
	// 成长/货币/武器盒。挂载本身对所有角色无害，是否生效由服务端按角色判定。
	mountEnvTable(env, project, "DFO_ODYSSEY_COIN_RULES", at("configs", "odyssey-currency.json"), nil)
	mountEnvTable(env, project, "DFO_ODYSSEY_WEAPON_BOX", at("configs", "odyssey-weapon-box-release.json"),
		map[string]string{"DFO_ODYSSEY_REWARDS_RELEASE": "1"})
	mountEnvTable(env, project, "DFO_ODYSSEY_GROWTH", at("configs", "odyssey-growth-release.json"), nil)
	mountEnvTable(env, project, "DFO_ODYSSEY_CHAPTERS", at("configs", "odyssey-chapters-release.json"), nil)
	mountEnvTable(env, project, "DFO_ODYSSEY_CHAPTER_DROP", at("configs", "odyssey-chapter-drop-release.json"), nil)

	if fullWear := at("configs", "equipment-wear.full-candidate.json"); regularFile(fullWear) {
		env.Set("DFO_EQUIPMENT_WEAR_RULES", fullWear)
	}

	// 下发前按当前服务端程序自报的能力过滤参数（channel_probe.py L552-L553）。
	command, dropped, err := PruneCommand(command, in.Probe(command[0]), env.Get("DFO_PVF_CATALOGS") != "")
	if err != nil {
		return result, err
	}
	if warning := DropWarning(command[0], dropped); warning != "" {
		result.Notices = append(result.Notices, warning)
	}
	// 当前源码走预置 PVF 域；历史二进制必须显式给出配套导出的 JSON。
	supported := in.Probe(command[0])
	if supported == nil {
		supported = map[string]bool{}
	}
	if supported["pvf-catalogs"] {
		env.Set("DFO_EQUIPMENT_CATALOG", "pvf")
		command = SetOptionValue(command, "-quest-equipment-catalog", "pvf")
	} else if historical := env.Get("DFO_HISTORICAL_EQUIPMENT_CATALOG"); historical != "" {
		command = SetOptionValue(command, "-quest-equipment-catalog", historical)
		env.Set("DFO_EQUIPMENT_CATALOG", historical)
	} else if in.Session.Candidate35 || in.Session.Candidate36 || in.Session.Candidate37 {
		return result, fmt.Errorf(
			"历史二进制需要 DFO_HISTORICAL_EQUIPMENT_CATALOG 与配套历史配置；请用 DFO_SERVER_BINARY 指定当前支持原生 PVF 的源码程序")
	}
	// 三张表都只有绝对路径找得到（网关 cwd 是包根）。
	env.Set("DFO_ATTUNEMENT_REWARDS", at("configs", "attunement-rewards.generated.json"))
	env.Set("DFO_EQUIPMENT_JOURNAL_RULES", at("configs", "equipment-journal.generated.json"))
	env.Set("DFO_EQUIPMENT_CREATE_COST", at("configs", "equipment-create-cost.generated.json"))
	result.Args = command
	return result, nil
}

// replaceFlagValue 对应 Python 的 command[command.index(flag) + 1] = value。flag 不在
// 命令行里时 Python 抛 ValueError（启动失败），这里同样报错而不是悄悄追加。
func replaceFlagValue(command []string, flag, value string) ([]string, error) {
	for index, token := range command {
		if token == flag {
			if index+1 >= len(command) {
				return nil, fmt.Errorf("命令行里 %s 后面没有值", flag)
			}
			command[index+1] = value
			return command, nil
		}
	}
	return nil, fmt.Errorf("命令行里没有 %s，无法用环境变量覆盖", flag)
}

// commandFlagValue 读一个 flag 的值，缺失返回 ""。
func commandFlagValue(command []string, flag string) string {
	for index, token := range command {
		if token == flag && index+1 < len(command) {
			return command[index+1]
		}
	}
	return ""
}

// hasAnySuffix 是 Python 的 tag.endswith((a, b, ...))。
func hasAnySuffix(value string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(value, suffix) {
			return true
		}
	}
	return false
}

// mountEnvTable 实现 channel_probe.py 的"默认表常驻挂载"（L490-L547）：环境变量已经
// 指了就用它（相对路径按包根解析，且必须真实存在），否则回落到出厂默认表 —— 同样要求
// 存在。extra 只在真正挂上时一起写入（武器盒要连带打开奖励开关）。
func mountEnvTable(env *childEnv, project, key, fallback string, extra map[string]string) {
	target := ""
	if current := env.Get(key); current != "" {
		if !filepath.IsAbs(current) {
			current = filepath.Join(project, current)
		}
		if regularFile(current) {
			target = filepath.Clean(current)
		}
	} else if regularFile(fallback) {
		target = fallback
	}
	if target == "" {
		return
	}
	env.Set(key, target)
	for extraKey, extraValue := range extra {
		env.Set(extraKey, extraValue)
	}
}

// validateJSONCatalogs 是 catalog_startup.py validate_json_catalogs 的移植：只在所选域
// **没有**走原生 PVF 时校验那份 JSON 导出。返回的告警原样转述给用户。
func validateJSONCatalogs(command []string, env *childEnv) ([]string, error) {
	usesPVFCatalog := func(domain string) bool {
		for _, part := range strings.Split(env.Get("DFO_PVF_CATALOGS"), ",") {
			if strings.TrimSpace(part) == domain {
				return true
			}
		}
		return false
	}
	var warnings []string

	if catalog := commandFlagValue(command, "-character-catalog"); catalog != "" && !usesPVFCatalog("characters") {
		if !regularFile(catalog) {
			warnings = append(warnings, fmt.Sprintf("WARNING: 角色目录不存在：%s", catalog))
		} else if growth, err := characterCatalogHasGrowth(catalog); err != nil {
			warnings = append(warnings, fmt.Sprintf("WARNING: 无法解析角色目录 %s：%v", catalog, err))
		} else if !growth {
			warnings = append(warnings, fmt.Sprintf(
				"WARNING: 角色目录 %s 不含 growtype 分段数据（advancement_growth/advancement_skills）："+
					"建号选定的转职分支不会落账，角色会停在基础职业。"+
					"请改用带该数据的目录（例如 configs/characters.skycastle-release.json）。", catalog))
		}
	}
	if commandFlagValue(command, "-dungeon-catalog") != "" && !usesPVFCatalog("dungeons") {
		return warnings, fmt.Errorf("副本内容已退休 JSON 入口，请使用当前源码程序并选择原生 PVF dungeons 域。")
	}
	return warnings, nil
}

// characterCatalogHasGrowth 读角色目录，判断是否有任意职业带 advancement_growth。
func characterCatalogHasGrowth(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	var catalog struct {
		Professions map[string]map[string]any `json:"professions"`
	}
	if err := json.Unmarshal(stripBOM(data), &catalog); err != nil {
		return false, err
	}
	for _, profession := range catalog.Professions {
		if pythonTruthy(profession["advancement_growth"]) {
			return true, nil
		}
	}
	return false, nil
}

// pythonTruthy 是 Python 的真值判断：None/0/""/{}/[]/False 为假，其余为真。角色目录里
// 的 advancement_growth 可能是对象或数组，Python 的 `(p or {}).get(...)` 判断的就是这个。
func pythonTruthy(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		return typed != ""
	case []any:
		return len(typed) > 0
	case map[string]any:
		return len(typed) > 0
	default:
		return true
	}
}
