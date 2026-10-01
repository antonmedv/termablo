<p align="center"><img src="demo/fields.gif" width="854" alt="A Fallen camp charges across the Ashen Fields"></p>

<h1 align="center">Termablo</h1>

<p align="center"><b>A Diablo-like roguelike for the terminal.</b><br>
Every torch, campfire, crystal, lava pool and spell casts real, colored, flickering light.<br>
You see only what the light reaches.</p>

## Play

Nothing to install:

```sh
ssh medv.io -p 2222
```

Or run it locally:

```sh
go run github.com/antonmedv/termablo@latest
```

Add `-seed 42` and the world is the same every time. Share the seed, race a friend.

<p align="center"><img src="demo/zones.png" width="854" alt="Emberhold, the Ashen Fields, the Throne of the Bone King, Blackmarsh, the Sunken Grotto and the Burning Abyss"></p>

## Host your own

```sh
docker compose up -d        # then: ssh localhost -p 2222
```

or, without Docker:

```sh
go run github.com/antonmedv/termablo@latest -ssh :2222
```

Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Wish](https://github.com/charmbracelet/wish).

## License

[MIT](LICENSE)
