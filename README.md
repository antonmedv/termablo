<p align="center"><img src="demo/fields.gif" width="854" alt="A Fallen camp charges across the Ashen Fields"></p>

**Termablo** is a Diablo-like roguelike for the terminal. Every torch, campfire, crystal, lava pool and spell casts real, colored, flickering light. You see only what the light reaches.

```sh
go run github.com/antonmedv/termablo@latest            # random world
go run github.com/antonmedv/termablo@latest -seed 42   # the same world every time
```

Needs a truecolor terminal, 80×24 or larger.

Host it over SSH, one game per connection:

```sh
go run . -ssh :2222     # then: ssh -p 2222 localhost
```

The host key is created in `.ssh/termablo_ed25519` (change with `-hostkey`).

Or with Docker, on port 2222 (`PORT=22 docker compose up -d` for another):

```sh
docker compose up -d
```

Emberhold, the last lit town before the dark. The Ashen Fields. The Crypt of the Fallen, where the Bone King waits. Blackmarsh, with cold lights over the water. The Sunken Grotto and the Drowned Oracle. Then the Burning Abyss, which keeps going down.

### Keys

| key | action |
|---|---|
| arrows / `hjkl` / numpad | move, attack |
| `f` `r` | Firebolt, Frost Nova |
| `tab` | cycle target |
| `q` `w` | healing potion, mana potion |
| `t` | town portal |
| `g` | pick up |
| `o` | auto-explore |
| `i` `c` `m` `?` | inventory, character, map, help |
| `Q` | quit |

[MIT](LICENSE)
