package main

type Tile uint8

const (
	TNone Tile = iota
	TFloor
	TWall
	TDoor
	TDoorOpen
	TStairsDown
	TStairsUp
	TGrass
	TDirt
	TRoad
	TWoodFloor
	TTree
	TDeadTree
	TRock
	TWater
	TDeepWater
	TBridge
	TLava
	TMud
	TReeds
	TCaveFloor
	TCaveWall
	TCrystal
	TBrazier
	TLamp
	TCampfire
	TFountain
	TGrave
	TPillar
	TChest
	TChestOpen
	TTownWall
	TBones
	TRubble
	TExit
	TCarpet
	TShelf
	TAltar
	TTileCount
)

// LightSpec describes a light source emitted by a tile, monster or item.
type LightSpec struct {
	Color     RGB
	Radius    float32
	Intensity float32
	Flicker   float32
	Pulse     float32
}

type TileDef struct {
	Name       string
	Glyphs     []rune
	Albedo     RGB
	BG         float32 // how strongly light tints the cell background
	BlockMove  bool
	BlockSight bool
	Emit       bool // glyph glows with its own color
	Emissive   RGB
	Anim       bool
	Light      *LightSpec
	Solid      bool // shown brightly in remembered map (walls)
}

var (
	lsBrazier  = &LightSpec{C(1, .5, .18), 8.5, 1.35, .22, 0}
	lsLamp     = &LightSpec{C(1, .72, .4), 7.5, 1.1, .08, 0}
	lsCampfire = &LightSpec{C(1, .42, .12), 10, 1.6, .3, 0}
	lsCrystal  = &LightSpec{C(.25, .5, 1), 7, 1.2, .03, .18}
	lsLava     = &LightSpec{C(1, .28, .04), 5.5, 1.0, .12, .1}
	lsAltar    = &LightSpec{C(.8, .25, 1), 5, .9, .05, .25}
	lsFountain = &LightSpec{C(.35, .7, .9), 3, .45, .02, .15}
)

var tdefs = [TTileCount]TileDef{
	TNone:       {Name: "void", Glyphs: []rune{' '}, BlockMove: true, BlockSight: true},
	TFloor:      {Name: "stone floor", Glyphs: []rune("·········.,·"), Albedo: C(.45, .42, .4), BG: .2},
	TWall:       {Name: "wall", Glyphs: []rune("#"), Albedo: C(.72, .66, .6), BG: .07, BlockMove: true, BlockSight: true, Solid: true},
	TDoor:       {Name: "door", Glyphs: []rune("+"), Albedo: C(.75, .48, .24), BG: .05, BlockMove: false, BlockSight: true, Solid: true},
	TDoorOpen:   {Name: "open door", Glyphs: []rune("'"), Albedo: C(.7, .45, .24), BG: .08},
	TStairsDown: {Name: "stairs down", Glyphs: []rune(">"), Albedo: C(1, .95, .85), BG: .12},
	TStairsUp:   {Name: "stairs up", Glyphs: []rune("<"), Albedo: C(1, .95, .85), BG: .12},
	TGrass:      {Name: "grass", Glyphs: []rune(",'`\".,··,'··"), Albedo: C(.32, .55, .24), BG: .07},
	TDirt:       {Name: "dirt", Glyphs: []rune("··.,··"), Albedo: C(.52, .42, .3), BG: .09},
	TRoad:       {Name: "cobblestones", Glyphs: []rune("··:·∙·"), Albedo: C(.58, .54, .5), BG: .11},
	TWoodFloor:  {Name: "wooden floor", Glyphs: []rune("··_·"), Albedo: C(.6, .42, .25), BG: .12},
	TTree:       {Name: "tree", Glyphs: []rune("♣♠♣♣"), Albedo: C(.2, .52, .22), BG: .03, BlockMove: true, BlockSight: true},
	TDeadTree:   {Name: "dead tree", Glyphs: []rune("τƒτ"), Albedo: C(.5, .42, .34), BG: .03, BlockMove: true},
	TRock:       {Name: "boulder", Glyphs: []rune("oO"), Albedo: C(.55, .54, .52), BG: .05, BlockMove: true},
	TWater:      {Name: "shallow water", Glyphs: []rune("~≈~ ~"), Albedo: C(.3, .45, .8), BG: .18, Anim: true},
	TDeepWater:  {Name: "deep water", Glyphs: []rune("≈~≈≈"), Albedo: C(.18, .3, .7), BG: .2, BlockMove: true, Anim: true},
	TBridge:     {Name: "bridge", Glyphs: []rune("="), Albedo: C(.6, .42, .24), BG: .1},
	TLava:       {Name: "lava", Glyphs: []rune("≈~≈~"), Albedo: C(1, .4, .1), BG: .5, BlockMove: true, Emit: true, Emissive: C(1, .38, .06), Anim: true, Light: lsLava},
	TMud:        {Name: "mud", Glyphs: []rune(".,··~"), Albedo: C(.38, .3, .2), BG: .08},
	TReeds:      {Name: "reeds", Glyphs: []rune("\"'\""), Albedo: C(.45, .55, .28), BG: .05},
	TCaveFloor:  {Name: "cave floor", Glyphs: []rune("··.·,··"), Albedo: C(.44, .39, .35), BG: .18},
	TCaveWall:   {Name: "cave wall", Glyphs: []rune("▓"), Albedo: C(.5, .44, .38), BG: .05, BlockMove: true, BlockSight: true, Solid: true},
	TCrystal:    {Name: "glowing crystal", Glyphs: []rune("♦"), Albedo: C(.5, .75, 1), BG: .25, BlockMove: true, BlockSight: true, Emit: true, Emissive: C(.5, .78, 1), Light: lsCrystal, Solid: true},
	TBrazier:    {Name: "brazier", Glyphs: []rune("Ψ"), Albedo: C(1, .6, .25), BG: .3, BlockMove: true, Emit: true, Emissive: C(1, .62, .22), Light: lsBrazier},
	TLamp:       {Name: "street lamp", Glyphs: []rune("¥"), Albedo: C(1, .8, .45), BG: .25, BlockMove: true, Emit: true, Emissive: C(1, .85, .5), Light: lsLamp},
	TCampfire:   {Name: "campfire", Glyphs: []rune("☼"), Albedo: C(1, .5, .15), BG: .4, BlockMove: true, Emit: true, Emissive: C(1, .55, .15), Light: lsCampfire},
	TFountain:   {Name: "fountain", Glyphs: []rune("¤"), Albedo: C(.5, .75, .95), BG: .2, BlockMove: true, Light: lsFountain},
	TGrave:      {Name: "grave", Glyphs: []rune("†"), Albedo: C(.65, .65, .68), BG: .05, BlockMove: true},
	TPillar:     {Name: "pillar", Glyphs: []rune("█"), Albedo: C(.55, .52, .48), BG: .05, BlockMove: true, BlockSight: true, Solid: true},
	TChest:      {Name: "chest", Glyphs: []rune("■"), Albedo: C(.9, .65, .28), BG: .1, BlockMove: true},
	TChestOpen:  {Name: "open chest", Glyphs: []rune("□"), Albedo: C(.55, .42, .22), BG: .08, BlockMove: true},
	TTownWall:   {Name: "town wall", Glyphs: []rune("#"), Albedo: C(.66, .62, .58), BG: .06, BlockMove: true, BlockSight: true, Solid: true},
	TBones:      {Name: "bones", Glyphs: []rune(";%;"), Albedo: C(.72, .7, .62), BG: .08},
	TRubble:     {Name: "rubble", Glyphs: []rune("°∙°"), Albedo: C(.5, .47, .43), BG: .08},
	TExit:       {Name: "path onward", Glyphs: []rune("·"), Albedo: C(.8, .72, .55), BG: .14},
	TCarpet:     {Name: "carpet", Glyphs: []rune("░"), Albedo: C(.55, .12, .12), BG: .1},
	TShelf:      {Name: "shelf", Glyphs: []rune("≡"), Albedo: C(.6, .45, .3), BG: .06, BlockMove: true},
	TAltar:      {Name: "blood altar", Glyphs: []rune("Ω"), Albedo: C(.9, .3, 1), BG: .3, BlockMove: true, Emit: true, Emissive: C(.85, .35, 1), Light: lsAltar},
}
