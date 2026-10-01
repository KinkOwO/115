package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"regexp"
	"strconv"
)

type OdysseyJournalNode struct {
	Destination [4]uint32 `json:"destination"`
	Dungeons    []uint32  `json:"dungeons"`
}

type OdysseyJournalRoutes struct {
	Source pvf.ArchiveSnapshot  `json:"source"`
	Path   string               `json:"path"`
	SHA256 string               `json:"sha256"`
	Nodes  []OdysseyJournalNode `json:"nodes"`
}

func NewOdysseyJournalRoutes(r OdysseyJournalRoutes) (*OdysseyJournalRoutes, error) {
	if r.Source.Checksum != OdysseySource || r.Path != OdysseyJournalPath || r.SHA256 != OdysseyChaptersJournalSHA {
		return nil, fmt.Errorf("Odyssey journal routes source mismatch")
	}
	if len(r.Nodes) != 29 {
		return nil, fmt.Errorf("Odyssey journal must have 29 nodes")
	}
	r.Nodes = append([]OdysseyJournalNode(nil), r.Nodes...)
	seen := map[uint32]bool{}
	for i, node := range r.Nodes {
		if node.Destination[0] == 0 || node.Destination[2] > math.MaxUint16 || node.Destination[3] > math.MaxUint16 || len(node.Dungeons) == 0 {
			return nil, fmt.Errorf("invalid Odyssey journal node %d", i)
		}
		r.Nodes[i].Dungeons = append([]uint32(nil), node.Dungeons...)
		for _, id := range node.Dungeons {
			if id == 0 || seen[id] {
				return nil, fmt.Errorf("duplicate or invalid Odyssey journal dungeon %d", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 50 {
		return nil, fmt.Errorf("Odyssey journal routes must contain 50 dungeons")
	}
	return &r, nil
}

func ImportOdysseyJournalRoutes(a *pvf.Archive) (*OdysseyJournalRoutes, error) {
	if a == nil {
		return nil, fmt.Errorf("Odyssey journal routes require PVF")
	}
	raw, err := a.ReadRaw(OdysseyJournalPath)
	if err != nil {
		return nil, err
	}
	text, err := a.ReadText(OdysseyJournalPath)
	if err != nil {
		return nil, err
	}
	nodes, err := parseOdysseyJournalRoutes(text)
	if err != nil {
		return nil, err
	}
	return NewOdysseyJournalRoutes(OdysseyJournalRoutes{Source: a.Snapshot(), Path: OdysseyJournalPath, SHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), Nodes: nodes})
}

var (
	odysseyNodeBlock    = regexp.MustCompile(`(?s)\[node\](.*?)\[/node\]`)
	odysseyNodeTeleport = regexp.MustCompile(`(?m)^\s*\[teleport info\][ \t]+(\d+)[ \t]+(\d+)[ \t]+(\d+)[ \t]+(\d+)[ \t\r]*$`)
)

func parseOdysseyJournalRoutes(text string) ([]OdysseyJournalNode, error) {
	var nodes []OdysseyJournalNode
	for _, block := range odysseyNodeBlock.FindAllStringSubmatch(text, -1) {
		var node OdysseyJournalNode
		positions := odysseyNodeTeleport.FindAllStringSubmatch(block[1], -1)
		if len(positions) != 1 {
			return nil, fmt.Errorf("Odyssey node has %d teleport definitions", len(positions))
		}
		for j := range node.Destination {
			v, err := strconv.ParseUint(positions[0][j+1], 10, 32)
			if err != nil {
				return nil, err
			}
			node.Destination[j] = uint32(v)
		}
		for _, dungeon := range dungeonBlock.FindAllStringSubmatch(block[1], -1) {
			m := indexLine.FindStringSubmatch(dungeon[1])
			if m == nil {
				return nil, fmt.Errorf("Odyssey node dungeon has no index")
			}
			id, err := strconv.ParseUint(m[1], 10, 32)
			if err != nil || id == 0 {
				return nil, fmt.Errorf("invalid Odyssey node dungeon index")
			}
			node.Dungeons = append(node.Dungeons, uint32(id))
		}
		if len(node.Dungeons) == 0 {
			return nil, fmt.Errorf("Odyssey node has no dungeons")
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}
