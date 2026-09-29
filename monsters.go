package main

import (
	"fmt"
	"math/rand"
)

type AIKind int

const (
	AIMelee AIKind = iota
	AIRanged
	AINPC
	AIBoneKing
	AIOracle
)

type MTemplate struct {
	ID        string
	Name      string
	Glyph     rune
	Color     RGB
	HP        int
	MinD      int
	MaxD      int
	Armor     int
	Speed     int
	XP        int
	AI        AIKind
	Range     int
	Erratic   int
	Dodge     int // percent chance to sidestep a firebolt
	Light     *LightSpec
	ProjColor RGB
	ProjGlyph rune
	ProjLight bool
	Pack      [2]int
	Undead    bool
	Verb      string
}

var mtList = []*MTemplate{
	{ID: "rat", Name: "Plague Rat", Glyph: 'r', Color: C(.7, .55, .42), HP: 6, MinD: 1, MaxD: 3, Speed: 120, XP: 4, Pack: [2]int{2, 5}, Verb: "bites"},
	{ID: "bat", Name: "Cave Bat", Glyph: 'b', Color: C(.7, .5, .62), HP: 5, MinD: 1, MaxD: 3, Dodge: 35, Speed: 150, XP: 4, Erratic: 45, Pack: [2]int{2, 4}, Verb: "bites"},
	{ID: "fallen", Name: "Fallen One", Glyph: 'f', Color: C(.95, .32, .25), HP: 8, MinD: 1, MaxD: 3, Speed: 100, XP: 6, Pack: [2]int{2, 5}, Verb: "stabs"},
	{ID: "shaman", Name: "Fallen Shaman", Glyph: 'f', Color: C(1, .7, .25), HP: 10, MinD: 2, MaxD: 5, Speed: 100, XP: 11, AI: AIRanged, Range: 6, ProjColor: C(1, .5, .15), ProjGlyph: '*', ProjLight: true, Pack: [2]int{1, 1}, Verb: "hurls fire at"},
	{ID: "zombie", Name: "Zombie", Glyph: 'z', Color: C(.5, .7, .38), HP: 18, MinD: 2, MaxD: 5, Speed: 70, XP: 8, Pack: [2]int{1, 3}, Undead: true, Verb: "claws"},
	{ID: "wolf", Name: "Dire Wolf", Glyph: 'C', Color: C(.68, .68, .72), HP: 12, MinD: 2, MaxD: 5, Dodge: 20, Speed: 130, XP: 9, Pack: [2]int{2, 4}, Verb: "mauls"},
	{ID: "skel", Name: "Skeleton", Glyph: 's', Color: C(.95, .92, .8), HP: 14, MinD: 2, MaxD: 6, Armor: 6, Speed: 100, XP: 10, Pack: [2]int{2, 4}, Undead: true, Verb: "slashes"},
	{ID: "archer", Name: "Skeleton Archer", Glyph: 's', Color: C(.7, .8, 1), HP: 10, MinD: 2, MaxD: 5, Speed: 100, XP: 12, AI: AIRanged, Range: 8, ProjColor: C(.85, .8, .7), ProjGlyph: '-', Pack: [2]int{1, 2}, Undead: true, Verb: "shoots"},
	{ID: "ghoul", Name: "Ghoul", Glyph: 'G', Color: C(.55, .75, .5), HP: 30, MinD: 3, MaxD: 8, Speed: 95, XP: 18, Pack: [2]int{1, 2}, Undead: true, Verb: "rends"},
	{ID: "cultist", Name: "Ember Cultist", Glyph: 'c', Color: C(.85, .4, .95), HP: 16, MinD: 3, MaxD: 6, Speed: 100, XP: 16, AI: AIRanged, Range: 7, ProjColor: C(1, .45, .12), ProjGlyph: '*', ProjLight: true, Pack: [2]int{1, 3}, Verb: "casts fire at"},
	{ID: "wraith", Name: "Wraith", Glyph: 'W', Color: C(.65, .8, 1), HP: 26, MinD: 4, MaxD: 9, Speed: 115, XP: 26, Light: &LightSpec{C(.4, .55, 1), 2.8, .55, .05, .2}, Pack: [2]int{1, 2}, Undead: true, Verb: "chills"},
	{ID: "wisp", Name: "Will-o'-Wisp", Glyph: 'w', Color: C(.6, .95, 1), HP: 10, MinD: 2, MaxD: 6, Dodge: 30, Speed: 130, XP: 14, Erratic: 40, Light: &LightSpec{C(.3, .75, 1), 4, .9, .1, .25}, Pack: [2]int{1, 3}, Verb: "shocks"},
	{ID: "drowned", Name: "Drowned Dead", Glyph: 'z', Color: C(.4, .65, .7), HP: 28, MinD: 3, MaxD: 8, Speed: 80, XP: 18, Pack: [2]int{2, 3}, Undead: true, Verb: "grasps"},
	{ID: "horror", Name: "Marsh Horror", Glyph: 'M', Color: C(.45, .6, .32), HP: 45, MinD: 5, MaxD: 12, Speed: 90, XP: 35, Pack: [2]int{1, 1}, Verb: "crushes"},
	{ID: "spider", Name: "Grotto Spider", Glyph: 'x', Color: C(.75, .62, .42), HP: 16, MinD: 3, MaxD: 7, Dodge: 15, Speed: 130, XP: 16, Pack: [2]int{3, 5}, Verb: "bites"},
	{ID: "golem", Name: "Crystal Golem", Glyph: 'g', Color: C(.6, .85, 1), HP: 55, MinD: 6, MaxD: 12, Armor: 20, Speed: 80, XP: 45, Light: &LightSpec{C(.3, .55, 1), 3.5, .8, .02, .15}, Pack: [2]int{1, 1}, Verb: "pummels"},
	{ID: "imp", Name: "Fire Imp", Glyph: 'i', Color: C(1, .6, .22), HP: 20, MinD: 3, MaxD: 7, Dodge: 20, Speed: 110, XP: 25, AI: AIRanged, Range: 6, ProjColor: C(1, .45, .1), ProjGlyph: '*', ProjLight: true, Light: &LightSpec{C(1, .45, .12), 3, .8, .3, 0}, Pack: [2]int{2, 4}, Verb: "spits fire at"},
	{ID: "hellspawn", Name: "Hellspawn", Glyph: 'H', Color: C(.95, .28, .22), HP: 70, MinD: 7, MaxD: 16, Armor: 15, Speed: 100, XP: 60, Light: &LightSpec{C(1, .2, .08), 2.5, .6, .2, 0}, Pack: [2]int{1, 2}, Verb: "cleaves"},

	{ID: "boneking", Name: "The Bone King", Glyph: 'K', Color: C(1, 1, .75), HP: 150, MinD: 7, MaxD: 14, Armor: 25, Speed: 100, XP: 500, AI: AIBoneKing, Light: &LightSpec{C(.5, 1, .45), 4.5, 1, .08, .2}, Undead: true, Verb: "smites"},
	{ID: "oracle", Name: "The Drowned Oracle", Glyph: 'O', Color: C(.45, .9, 1), HP: 170, MinD: 6, MaxD: 12, Armor: 20, Speed: 100, XP: 1100, AI: AIOracle, Range: 8, ProjColor: C(.45, .8, 1), ProjGlyph: '*', ProjLight: true, Light: &LightSpec{C(.3, .8, 1), 5.5, 1, .05, .3}, Verb: "drowns"},

	{ID: "smith", Name: "Hadrik the Smith", Glyph: '@', Color: C(1, .6, .3), AI: AINPC, HP: 999, Speed: 100},
	{ID: "alch", Name: "Old Mirela", Glyph: '@', Color: C(.55, .85, .95), AI: AINPC, HP: 999, Speed: 100},
	{ID: "captain", Name: "Captain Voss", Glyph: '@', Color: C(.95, .9, .55), AI: AINPC, HP: 999, Speed: 100},
	{ID: "villager", Name: "Villager", Glyph: '@', Color: C(.6, .55, .5), AI: AINPC, HP: 999, Speed: 100},
}

var mtemps = func() map[string]*MTemplate {
	m := map[string]*MTemplate{}
	for _, t := range mtList {
		m[t.ID] = t
	}
	return m
}()

type Mod int

const (
	ModStrong Mod = iota
	ModSwift
	ModStone
	ModFire
	ModVampiric
	ModCount
)

var modNames = [ModCount]string{"Extra Strong", "Swift", "Stone Skin", "Fire Enchanted", "Vampiric"}

const (
	RankNormal = iota
	RankChampion
	RankUnique
	RankBoss
)

type Monster struct {
	T         *MTemplate
	Name      string
	X, Y      int
	HP, MaxHP int
	MinD      int
	MaxD      int
	Armor     int
	ToHit     int
	Level     int
	Speed     int
	Energy    int
	XP        int
	Rank      int
	Mods      []Mod
	Awake     bool
	LostTurns int
	Frozen    int
	FreezeCD  int // turns until nova can freeze it again
	Light     *Light
	Dead      bool
	Friendly  bool
	Flash     float64
	HomeX     int
	HomeY     int
	Timer     int
	Minion    bool
	Talk      int
	Flee      int
}

func (m *Monster) HasMod(md Mod) bool {
	for _, x := range m.Mods {
		if x == md {
			return true
		}
	}
	return false
}

func (m *Monster) Color() RGB {
	if m.Rank == RankChampion {
		return m.T.Color.Lerp(C(.45, .6, 1), .45)
	}
	if m.Rank == RankUnique {
		return m.T.Color.Lerp(colGold, .5)
	}
	return m.T.Color
}

var uniqueFirst = []string{"Bloodmaw", "Grimtooth", "Ashfang", "Hollowgrin", "Rotgut", "Blackvein", "Skullcleave", "Mournwind", "Gravelurk", "Emberskull", "Frostmarrow", "Dreadhowl", "Cinderjaw", "Wormheart"}
var uniqueLast = []string{"the Hungry", "the Vile", "the Unburied", "the Defiler", "the Pale", "the Cruel", "the Wretched", "the Weeping", "the Gnawer"}

func NewMonster(rng *rand.Rand, t *MTemplate, lvl, rank int) *Monster {
	if lvl < 1 {
		lvl = 1
	}
	// Life grows slowly at first, then steeply, so the fields stay gentle
	// and the deep crypt keeps pace with a geared hero.
	l1 := float64(lvl - 1)
	hpMul := 1 + 0.32*l1 + 0.05*l1*l1
	dMul := 1 + 0.24*l1 + 0.02*l1*l1
	if t.AI == AIBoneKing || t.AI == AIOracle {
		rank = RankBoss
		hpMul = 1 + 0.22*float64(lvl-1)
	}
	m := &Monster{T: t, Name: t.Name, Level: lvl, Rank: rank, Speed: t.Speed}
	hp := float64(t.HP) * hpMul
	xp := float64(t.XP) * (1 + 0.45*float64(lvl-1))
	switch rank {
	case RankChampion:
		hp *= 2.2
		dMul *= 1.3
		xp *= 3
	case RankUnique:
		hp *= 3
		dMul *= 1.3
		xp *= 6
		m.Name = uniqueFirst[rng.Intn(len(uniqueFirst))] + " " + uniqueLast[rng.Intn(len(uniqueLast))]
	}
	m.MaxHP = int(hp)
	if t.AI == AINPC {
		m.MaxHP = 999
		m.Friendly = true
	}
	m.HP = m.MaxHP
	m.MinD = maxi(1, int(float64(t.MinD)*dMul))
	m.MaxD = maxi(m.MinD, int(float64(t.MaxD)*dMul))
	m.Armor = t.Armor + lvl*2
	m.ToHit = 55 + lvl*3
	m.XP = int(xp)
	if rank == RankChampion || rank == RankUnique {
		n := 1
		if rank == RankUnique {
			n = 1 + mini(2, lvl/3)
		}
		for len(m.Mods) < n {
			md := Mod(rng.Intn(int(ModCount)))
			if !m.HasMod(md) {
				m.Mods = append(m.Mods, md)
			}
		}
		for _, md := range m.Mods {
			switch md {
			case ModStrong:
				m.MinD, m.MaxD = m.MinD*3/2, m.MaxD*3/2
			case ModSwift:
				m.Speed += 40
			case ModStone:
				m.Armor += 30 + lvl*4
			}
		}
	}
	spec := t.Light
	if m.HasMod(ModFire) {
		spec = &LightSpec{C(1, .45, .12), 3.2, .9, .3, 0}
	}
	if spec != nil {
		m.Light = NewLight(0, 0, spec, rng.Float32()*100)
	}
	m.Energy = rng.Intn(100)
	m.Flash = -10
	return m
}

func (m *Monster) Describe() string {
	if len(m.Mods) == 0 {
		return ""
	}
	s := ""
	for i, md := range m.Mods {
		if i > 0 {
			s += ", "
		}
		s += modNames[md]
	}
	return s
}

// spawn tables per region
type spawnEntry struct {
	id     string
	w      int
	minLvl int
}

var spawnTables = map[string][]spawnEntry{
	"fields": {{"rat", 4, 0}, {"fallen", 5, 0}, {"shaman", 2, 0}, {"zombie", 3, 0}, {"wolf", 3, 1}, {"bat", 2, 0}},
	"crypt":  {{"zombie", 4, 0}, {"skel", 5, 0}, {"archer", 3, 0}, {"fallen", 3, 0}, {"shaman", 1, 0}, {"bat", 2, 0}, {"ghoul", 3, 3}, {"cultist", 3, 3}, {"wraith", 2, 4}},
	"marsh":  {{"wisp", 4, 0}, {"drowned", 5, 0}, {"horror", 2, 0}, {"bat", 2, 0}, {"cultist", 2, 0}, {"wolf", 2, 0}},
	"grotto": {{"spider", 5, 0}, {"golem", 2, 0}, {"wisp", 3, 0}, {"drowned", 3, 0}, {"ghoul", 2, 0}, {"wraith", 2, 7}},
	"abyss":  {{"imp", 5, 0}, {"hellspawn", 3, 0}, {"wraith", 3, 0}, {"skel", 2, 0}, {"cultist", 3, 0}, {"golem", 1, 0}, {"spider", 2, 0}},
}

func pickSpawn(rng *rand.Rand, table string, lvl int) *MTemplate {
	entries := spawnTables[table]
	tot := 0
	for _, e := range entries {
		if lvl >= e.minLvl {
			tot += e.w
		}
	}
	r := rng.Intn(tot)
	for _, e := range entries {
		if lvl < e.minLvl {
			continue
		}
		if r < e.w {
			return mtemps[e.id]
		}
		r -= e.w
	}
	return mtemps[entries[0].id]
}

// populate places monster packs across a level, keeping clear of safe spots.
func populate(l *Level, table string, packs int, safe []Pos) {
	rng := l.rng
	reach := l.reachable(l.Start.X, l.Start.Y)
	tooClose := func(x, y int) bool {
		for _, s := range safe {
			if cheb(x, y, s.X, s.Y) < 12 {
				return true
			}
		}
		return false
	}
	uniquePlaced := false
	for range packs {
		var x, y int
		ok := false
		for range 200 {
			x, y = rng.Intn(l.W), rng.Intn(l.H)
			if reach[l.Idx(x, y)] && l.Walkable(x, y) && !tooClose(x, y) && l.MonsterAt(x, y) == nil && l.LinkAt(x, y) == nil {
				ok = true
				break
			}
		}
		if !ok {
			continue
		}
		t := pickSpawn(rng, table, l.Depth)
		lvl := l.Depth
		if rng.Intn(3) == 0 {
			lvl++
		}
		n := t.Pack[0] + rng.Intn(t.Pack[1]-t.Pack[0]+1)
		leaderRank := RankNormal
		if !uniquePlaced && rng.Intn(100) < 30 {
			leaderRank = RankUnique
			uniquePlaced = true
			n += 2
		} else if rng.Intn(100) < 8+3*l.Depth { // champions grow common deeper
			leaderRank = RankChampion
			n = maxi(n, 2)
		}
		for i := range n {
			rank := RankNormal
			if leaderRank == RankChampion {
				rank = RankChampion
			} else if leaderRank == RankUnique && i == 0 {
				rank = RankUnique
			}
			mx, my := l.FreeNear(x, y, -1, -1)
			if !reach[l.Idx(mx, my)] {
				continue
			}
			m := NewMonster(rng, t, lvl, rank)
			m.X, m.Y, m.HomeX, m.HomeY = mx, my, mx, my
			if rank == RankUnique {
				m.Minion = false
			}
			l.Monsters = append(l.Monsters, m)
		}
	}
}

func placeMonster(l *Level, id string, x, y, lvl, rank int) *Monster {
	return placeMonsterAvoid(l, id, x, y, lvl, rank, -1, -1)
}

func placeMonsterAvoid(l *Level, id string, x, y, lvl, rank, px, py int) *Monster {
	m := NewMonster(l.rng, mtemps[id], lvl, rank)
	m.X, m.Y = l.FreeNear(x, y, px, py)
	m.HomeX, m.HomeY = m.X, m.Y
	l.Monsters = append(l.Monsters, m)
	return m
}

func (m *Monster) String() string { return fmt.Sprintf("%s(%d/%d)", m.Name, m.HP, m.MaxHP) }
