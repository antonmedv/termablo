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
	HealPool float64 // life still to come from drunk potions
	ManaPool float64
	Torch    *Light
	Kills    int
	KilledBy string
	Flash    float64
	st       [StCount]int
	r        *Rules
}

func NewPlayer(r *Rules) *Player {
	p := &Player{Lvl: 1, Str: 15, Dex: 15, Vit: 15, Ene: 15, Gold: 60, HPot: 3, MPot: 2, Scrolls: 1, Flash: -10, r: r}
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

func (p *Player) MaxHP() int {
	return 30 + p.VIT()*2 + int(float64(p.Lvl-1)*p.r.HPPerLvl) + p.st[StLife]
}
func (p *Player) MaxMP() int {
	return 14 + int(float64(p.ENE())*1.2) + (p.Lvl-1)*2 + p.st[StMana]
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
	mul := (1 + float64(p.STR())/p.r.StrDiv) * (1 + float64(p.st[StDmgPct])/100)
	return maxi(1, int(float64(lo)*mul)), maxi(1, int(math.Ceil(float64(hi)*mul)))
}

func (p *Player) Crit() int { return mini(75, 5+p.DEX()/10+p.st[StCrit]) }
func (p *Player) ToHit() int {
	return 70 + p.DEX()/4 + p.st[StToHit]/5 + int(float64(p.Lvl-1)*p.r.ToHitPerLvl)
}

func (p *Player) SpellMul() float64 { return 1 + float64(p.st[StSpellPct])/100 }

// Spells scale with Energy alone: a fighter who never invests in it keeps
// the starting Firebolt while monsters grow.
func (p *Player) FireboltDmg() (int, int) {
	r := p.r
	e, m := math.Pow(float64(p.ENE()), r.BoltEnePow), p.SpellMul()
	return int((5 + e*0.2) * m * r.BoltMul), int((9 + e*0.28) * m * r.BoltMul)
}

func (p *Player) NovaDmg() (int, int) {
	e, m := float64(p.ENE()), p.SpellMul()
	return int((3 + e*0.12) * m * p.r.NovaMul), int((6 + e*0.15) * m * p.r.NovaMul)
}

// Spells cost more as the caster grows, so a deeper mana pool does not
// simply mean more casts.
func (p *Player) FireboltCost() int { return costFirebolt + (p.Lvl-1)*2/3 }
func (p *Player) NovaCost() int     { return costNova + p.Lvl - 1 }

const (
	costFirebolt = 6
	costNova     = 14

	fireboltRange = 10 // steps

	beltMax = 5 // potions of each kind the belt holds

	// Nova freezes for a couple of turns, then the target shakes it off
	// for a while.
	novaFreeze   = 3
	freezeImmune = 6
)
