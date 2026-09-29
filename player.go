package main

import "math"

const invMax = 24

type Player struct {
	X, Y     int
	Lvl, XP  int
	Str, Dex int
	Vit, Ene int
	Points   int
	HP, MP   float64
	Gold     int
	Inv      []*Item
	Eq       [EqCount]*Item
	HPot     int
	MPot     int
	Scrolls  int
	Torch    *Light
	Kills    int
	KilledBy string
	Flash    float64
	st       [StCount]int
}

func NewPlayer() *Player {
	p := &Player{Lvl: 1, Str: 15, Dex: 15, Vit: 15, Ene: 15, Gold: 60, HPot: 3, MPot: 2, Scrolls: 1, Flash: -10}
	p.Torch = NewLight(0, 0, &LightSpec{C(1, .62, .3), 6.5, 1.3, .1, 0}, 3.3)
	p.recalc()
	p.HP, p.MP = float64(p.MaxHP()), float64(p.MaxMP())
	return p
}

func (p *Player) recalc() {
	p.st = [StCount]int{}
	for _, it := range p.Eq {
		if it == nil {
			continue
		}
		for _, a := range it.Aff {
			p.st[a.S] += a.V
		}
	}
	if p.HP > float64(p.MaxHP()) {
		p.HP = float64(p.MaxHP())
	}
	if p.MP > float64(p.MaxMP()) {
		p.MP = float64(p.MaxMP())
	}
	r := float32(6.5 + 0.7*float64(p.st[StLight]))
	if r < 3 {
		r = 3
	}
	if p.Torch.Radius != r {
		p.Torch.Radius = r
		p.Torch.ver = -1
	}
}

func (p *Player) S(s Stat) int { return p.st[s] }
func (p *Player) STR() int     { return p.Str + p.st[StStr] + p.st[StAllAttr] }
func (p *Player) DEX() int     { return p.Dex + p.st[StDex] + p.st[StAllAttr] }
func (p *Player) VIT() int     { return p.Vit + p.st[StVit] + p.st[StAllAttr] }
func (p *Player) ENE() int     { return p.Ene + p.st[StEne] + p.st[StAllAttr] }

func (p *Player) MaxHP() int { return 30 + p.VIT()*2 + (p.Lvl-1)*4 + p.st[StLife] }
func (p *Player) MaxMP() int {
	return 10 + int(float64(p.ENE())*1.5) + (p.Lvl-1)*2 + p.st[StMana]
}

func (p *Player) ArmorVal() int {
	a := p.st[StArmor] + p.DEX()/5
	for _, it := range p.Eq {
		if it != nil {
			a += it.Armor
		}
	}
	return a
}

func (p *Player) DmgRange() (int, int) {
	lo, hi := 1, 3
	if w := p.Eq[EqWeapon]; w != nil {
		lo, hi = w.MinD, w.MaxD
	}
	lo += p.st[StFlatDmg]
	hi += p.st[StFlatDmg]
	mul := (1 + float64(p.STR())/60) * (1 + float64(p.st[StDmgPct])/100)
	return maxi(1, int(float64(lo)*mul)), maxi(1, int(math.Ceil(float64(hi)*mul)))
}

func (p *Player) Crit() int  { return mini(75, 5+p.DEX()/10+p.st[StCrit]) }
func (p *Player) ToHit() int { return 70 + p.DEX()/4 + p.st[StToHit]/5 }

func (p *Player) SpellMul() float64 { return 1 + float64(p.st[StSpellPct])/100 }

func (p *Player) FireboltDmg() (int, int) {
	base := float64(p.Lvl) + float64(p.ENE())/3
	m := p.SpellMul()
	return int((2 + base) * m), int((6 + base*1.3) * m)
}

func (p *Player) NovaDmg() (int, int) {
	base := float64(p.Lvl)*0.8 + float64(p.ENE())/4
	m := p.SpellMul()
	return int((1 + base) * m), int((4 + base) * m)
}

const (
	costFirebolt = 6
	costNova     = 14
)

func xpNext(lvl int) int { return int(35 * math.Pow(float64(lvl), 1.8)) }
