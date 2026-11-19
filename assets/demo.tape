Output assets/demo.gif

Require nivis

Set Shell "bash"
Set FontFamily "IBM Plex Mono"
Set FontSize 17
Set Width 1280
Set Height 720
Set Padding 28
Set TypingSpeed 45ms
Set Framerate 10
Set Theme { "name": "nivis", "background": "#020713", "foreground": "#deeaf0", "cursor": "#2dd2c5", "selection": "#263a4e", "black": "#020713", "red": "#dc645a", "green": "#5ac878", "yellow": "#dcbe5a", "blue": "#7bd3ff", "magenta": "#9abaff", "cyan": "#5ce7df", "white": "#deeaf0", "brightBlack": "#93b8c7", "brightRed": "#dc645a", "brightGreen": "#5ac878", "brightYellow": "#dcbe5a", "brightBlue": "#7bd3ff", "brightMagenta": "#9abaff", "brightCyan": "#5ce7df", "brightWhite": "#ffffff" }

Hide
Type "export PS1='\[\e[38;2;45;210;197m\]$\[\e[0m\] '"
Enter
Type "clear"
Enter
Show

Type "nivis plan"
Sleep 300ms
Enter
Sleep 2.5s

Hide
Type "clear"
Enter
Show

Type "nivis apply"
Sleep 300ms
Enter
Sleep 5s

Type "nivis output"
Sleep 300ms
Enter
Sleep 3.5s
