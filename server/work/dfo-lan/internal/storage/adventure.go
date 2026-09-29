package storage

import (
	"context"
	"dfolan/internal/adventure"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/encoding/simplifiedchinese"
	"math"
	"time"
)

// 冒险团属于账号。独立建表，不改写已有角色 JSON 或角色编号。
type AccountAdventure struct {
	Name       string
	Level      uint32
	Experience uint64
	CreatedAt  time.Time
	Data       AdventureData
}

// AdventureLevel 只读账号等级；首次建立档案前使用源规则的初始等级。
// 不缓存到角色存档，避免同账号其他角色升级后继续发送旧等级。
func (s *Store) AdventureLevel(ctx context.Context, account, character int64) (uint32, error) {
	if !s.adventureEnabled {
		return 0, nil
	}
	var level uint32
	err := s.DB.QueryRow(ctx, `SELECT COALESCE(a.level,1) FROM characters c
 LEFT JOIN account_adventures a ON a.account_id=c.account_id
 WHERE c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL`, account, character).Scan(&level)
	return level, err
}

// CommitAdventure 将账号资料、背包变化及命令回执作为一次事务提交。
// 调用方传入会话和原始帧生成的幂等键；成功回包丢失后重试不重复扣款。
func (s *Store) CommitAdventure(ctx context.Context, account, id int64, key string, apply func(Character, *AccountAdventure) (json.RawMessage, json.RawMessage, error)) (Character, AccountAdventure, json.RawMessage, error) {
	var role Character
	var p AccountAdventure
	var receipt json.RawMessage
	if key == "" || len(key) > 200 || apply == nil {
		return role, p, nil, fmt.Errorf("冒险团事务参数无效")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return role, p, nil, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if err != nil {
		return role, p, nil, err
	}
	p, err = lockAdventure(ctx, tx, role)
	if err != nil {
		return role, p, nil, err
	}
	err = tx.QueryRow(ctx, `SELECT outcome FROM character_events WHERE character_id=$1 AND event_key=$2 AND model='account-adventure-v1'`, id, key).Scan(&receipt)
	if err == nil {
		return role, p, receipt, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return role, p, nil, err
	}
	state, receipt, err := apply(role, &p)
	if err != nil {
		return role, p, nil, err
	}
	if !json.Valid(state) || !json.Valid(receipt) {
		return role, p, nil, fmt.Errorf("冒险团事务结果无效")
	}
	if err = saveAdventure(ctx, tx, account, p); err != nil {
		return role, p, nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); err != nil {
		return role, p, nil, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,'account-adventure-v1',$4)`, id, key, role.ConfigVersion, receipt); err != nil {
		return role, p, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return role, p, nil, err
	}
	role.State = state
	if s.Cache != nil {
		s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	}
	return role, p, receipt, nil
}

// 账号共享的货币和限购记录不放在单个角色 JSON 中，避免换角色重复兑换。
type AdventureData struct {
	RecommendedDungeonClears uint32                `json:"recommended_dungeon_clears,omitempty"`
	SeasonLevel              adventure.SeasonState `json:"season_level,omitempty"`
	Points                   [5]uint32             `json:"points"`
	Purchases                map[uint32]uint32     `json:"purchases,omitempty"`
	PointExperience          uint64                `json:"point_experience,omitempty"`
	ExperienceRemainder      uint64                `json:"experience_remainder,omitempty"`
	Period                   string                `json:"period,omitempty"`
	LastConnectionDay        string                `json:"last_connection_day,omitempty"`
	ConnectionDays           uint32                `json:"connection_days,omitempty"`
	BestHonorCharacter       uint32                `json:"best_honor_character,omitempty"`
	EliteSelections          map[uint16][3]int64   `json:"elite_selections,omitempty"`
	// 使用稳定角色ID关联技能偏好，避免角色排序或精锐槽位变化后串用。
	// 老存档没有该字段时保持零值，不迁移或覆盖已有角色技能、装备。
	EliteSkillUsage map[uint16]map[int64][30]int32 `json:"elite_skill_usage,omitempty"`
}

func (s *Store) MigrateAdventure(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS account_adventures (
 account_id bigint PRIMARY KEY REFERENCES accounts(id),
 name text NOT NULL CHECK(length(name)>0),
 level integer NOT NULL DEFAULT 1 CHECK(level BETWEEN 1 AND 50),
 experience bigint NOT NULL DEFAULT 0 CHECK(experience>=0),
 created_at timestamptz NOT NULL DEFAULT now());
 ALTER TABLE account_adventures ADD COLUMN IF NOT EXISTS data jsonb NOT NULL DEFAULT '{}';`)
	if err == nil {
		_, err = adventure.Current()
	}
	if err == nil {
		s.adventureEnabled = true
	}
	return err
}

// 首次接入沿用账号建立日期和调用方校验过的默认团名。
// ON CONFLICT 不更新任何字段，重登、换角色及重复打开不会重置资料。
func (s *Store) LoadAdventure(ctx context.Context, account, character int64, defaultName string) (AccountAdventure, error) {
	var result AccountAdventure
	_, err := s.DB.Exec(ctx, `INSERT INTO account_adventures(account_id,name,created_at)
 SELECT a.id,$3,a.created_at FROM accounts a
 JOIN characters c ON c.account_id=a.id
 WHERE a.id=$1 AND c.id=$2 AND c.deleted_at IS NULL
 ON CONFLICT(account_id) DO NOTHING`, account, character, defaultName)
	if err != nil {
		return result, err
	}
	var raw []byte
	err = s.DB.QueryRow(ctx, `SELECT a.name,a.level,a.experience,a.created_at,a.data
 FROM account_adventures a JOIN characters c ON c.account_id=a.account_id
 WHERE a.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL`, account, character).Scan(
		&result.Name, &result.Level, &result.Experience, &result.CreatedAt, &raw)
	if err == nil {
		err = json.Unmarshal(raw, &result.Data)
	}
	return result, err
}

func adventureName(name string) string {
	out := ""
	for _, r := range name {
		next := out + string(r)
		b, e := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(next))
		if e != nil || len(b) > 16 {
			break
		}
		out = next
	}
	if out == "" {
		return "冒险团"
	}
	return out
}

// 沿用服务端游戏日历；月切只清理源商店标记的限购/积分，永久积分保留。
// 离线期间不依赖定时器，重新登录和访问时补做。
func (s *Store) PrepareAdventure(ctx context.Context, role Character, day string) (AccountAdventure, error) {
	var p AccountAdventure
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return p, err
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return p, err
	}
	defer tx.Rollback(ctx)
	var owned bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM characters WHERE id=$1 AND account_id=$2 AND deleted_at IS NULL)`, role.ID, role.AccountID).Scan(&owned)
	if err != nil {
		return p, err
	}
	if !owned {
		return p, fmt.Errorf("冒险团角色不属于当前账号")
	}
	p, err = lockAdventure(ctx, tx, role)
	if err != nil {
		return p, err
	}
	rules, err := adventure.Current()
	if err != nil {
		return p, err
	}
	changed := false
	// 与经验、商店共用账号行锁；同日换角色、重连及在线轮询只计一天。
	// 老档没有登录历史时从首次观察到的游戏日开始，不根据建号日期伪造连续天数。
	if p.Data.LastConnectionDay == "" || p.Data.LastConnectionDay < day {
		streak := uint32(1)
		if p.Data.LastConnectionDay != "" {
			previous, e := time.Parse("2006-01-02", p.Data.LastConnectionDay)
			if e != nil {
				return p, fmt.Errorf("冒险团上次登录日期无效：%w", e)
			}
			if previous.AddDate(0, 0, 1).Format("2006-01-02") == day {
				if p.Data.ConnectionDays >= math.MaxInt32 {
					return p, fmt.Errorf("冒险团连续登录天数超出客户端范围")
				}
				streak = p.Data.ConnectionDays + 1
			}
		}
		p.Data.LastConnectionDay, p.Data.ConnectionDays = day, streak
		changed = true
	}
	month := day[:7]
	if p.Data.Period < month {
		if p.Data.Period != "" {
			for category, shop := range rules.Shops {
				if shop.ResetPoints {
					p.Data.Points[category] = 0
				}
				for _, item := range shop.Items {
					if item.Reset == 3 {
						delete(p.Data.Purchases, item.Template)
					}
				}
			}
		}
		p.Data.Period = month
		changed = true
	}
	if changed {
		if err = saveAdventure(ctx, tx, role.AccountID, p); err != nil {
			return p, err
		}
	}
	return p, tx.Commit(ctx)
}

func lockAdventure(ctx context.Context, tx pgx.Tx, role Character) (AccountAdventure, error) {
	var p AccountAdventure
	var firstName string
	err := tx.QueryRow(ctx, `SELECT name FROM characters WHERE account_id=$1 AND deleted_at IS NULL ORDER BY id LIMIT 1`, role.AccountID).Scan(&firstName)
	if err != nil {
		return p, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO account_adventures(account_id,name,created_at)
 SELECT id,$2,created_at FROM accounts WHERE id=$1 ON CONFLICT DO NOTHING`, role.AccountID, adventureName(firstName))
	if err != nil {
		return p, err
	}
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT name,level,experience,created_at,data FROM account_adventures WHERE account_id=$1 FOR UPDATE`, role.AccountID).Scan(&p.Name, &p.Level, &p.Experience, &p.CreatedAt, &raw)
	if err == nil {
		err = json.Unmarshal(raw, &p.Data)
	}
	if p.Data.Purchases == nil {
		p.Data.Purchases = map[uint32]uint32{}
	}
	return p, err
}

func saveAdventure(ctx context.Context, tx pgx.Tx, account int64, p AccountAdventure) error {
	raw, err := json.Marshal(p.Data)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE account_adventures SET level=$2,experience=$3,data=$4 WHERE account_id=$1`, account, p.Level, p.Experience, raw)
	return err
}

// 调用方已经锁定角色；账号行随后加锁，所有角色共用相同锁序。
// 经验与角色、幂等回执一起提交，数据库失败时全部回滚。
func (s *Store) commitAdventureExperience(ctx context.Context, tx pgx.Tx, role Character, state json.RawMessage) error {
	if !s.adventureEnabled {
		return nil
	}
	var old, next struct {
		Earned uint64 `json:"adventure_earned_experience"`
	}
	if err := json.Unmarshal(role.State, &old); err != nil {
		return err
	}
	if err := json.Unmarshal(state, &next); err != nil {
		return err
	}
	if next.Earned < old.Earned {
		return fmt.Errorf("冒险团经验计数不允许回退")
	}
	gain := next.Earned - old.Earned
	if gain == 0 {
		return nil
	}
	rules, err := adventure.Current()
	if err != nil {
		return err
	}
	p, err := lockAdventure(ctx, tx, role)
	if err != nil {
		return err
	}
	// 源比例 0.3，保留十分位余数，连续小额击杀不丢经验。
	if rules.ExpRate != float32(0.3) || gain > (math.MaxUint64-p.Data.ExperienceRemainder)/3 {
		return fmt.Errorf("冒险团经验比例或增量无效")
	}
	scaled := gain*3 + p.Data.ExperienceRemainder
	earned := scaled / 10
	p.Data.ExperienceRemainder = scaled % 10
	if earned > math.MaxInt64-p.Experience {
		return fmt.Errorf("冒险团经验溢出")
	}
	p.Experience += earned
	// 原生 UI 直接计算当前经验 / 下级需求，不减去历史等级阈值。
	for p.Level < rules.MaxLevel && p.Experience >= rules.Experience[p.Level+1] {
		p.Experience -= rules.Experience[p.Level+1]
		p.Level++
	}
	if p.Level == rules.MaxLevel {
		p.Experience = 0
	}
	shop := rules.Shops[0]
	if len(shop.ExpPoints) != 2 || shop.ExpPoints[0] == 0 {
		return fmt.Errorf("冒险团经验积分规则缺失")
	}
	if earned > math.MaxUint64-p.Data.PointExperience {
		return fmt.Errorf("冒险团积分经验溢出")
	}
	p.Data.PointExperience += earned
	points := p.Data.PointExperience / shop.ExpPoints[0] * shop.ExpPoints[1]
	p.Data.PointExperience %= shop.ExpPoints[0]
	p.Data.Points[0] = uint32(min(uint64(shop.MaxPoints), uint64(p.Data.Points[0])+points))
	return saveAdventure(ctx, tx, role.AccountID, p)
}
