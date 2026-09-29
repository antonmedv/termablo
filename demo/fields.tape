# Ashen Fields: a Fallen camp charges out of the moonlight; Frost Nova, Firebolts.
# Render:  go build -o termablo . && vhs demo/fields.tape
# Then crop away the side panel and log (map region only) and quantize:
#   gifsicle -O3 --colors 128 --crop 0,0+854x461 -b demo/fields.gif
Output demo/fields.gif
Set Shell bash
Set FontSize 13
Set Columns 136
Set Rows 36
Set Padding 0
Set Framerate 10
Set TypingSpeed 100ms
Hide
Type "./termablo -seed 148 -level fields"
Enter
Sleep 800ms
Enter
Sleep 300ms
Show
Sleep 500ms
Type "lllllllll"
Sleep 400ms
Type@150ms "......"
Sleep 300ms
Type "r"
Sleep 900ms
Type@500ms "fff"
Sleep 200ms
Type "w"
Sleep 400ms
Type "f"
Sleep 500ms
Type@200ms "ll"
Sleep 1s
Hide
Type "Q"
