Sina-Live-Fetcher
=================

These are utilities to fetch HTTP live stream URL from [Sina Live Game Broadcast](http://kan.sina.com.cn).

## Compiling

Use your favorite Go compiler ([gc](https://golang.org/doc/install) or [gccgo](https://golang.org/doc/install/gccgo)) to compile it.

Note due to [a Wine bug](https://groups.google.com/d/msg/golang-nuts/nhJOw71rw7k/NTaK2_G794QJ), this program may not function properly under Wine. Build native Linux version if you run it on Linux.

## stream-url-fetcher

This program takes one argument, which is the web page URL of the live broadcast.

It fetches the URL of the broadcast and prints it out. You can start [a media player](http://mpv.io/) to play the live stream.

If there is no live broadcast at the moment, a video for a previous record will be used, which was exactly the one you will see if you visit the web page.

## live-comment-fetcher

This program takes one argument, which is the web page URL of the live broadcast.

It connects to the chatroom and prints out messages in realtime.

You can use pipe to feed the output to [live-danmaku-hime](https://github.com/m13253/live-danmaku-hime), which is a desktop widget that hangs on the right side of your desktop and displays comments from your audience when you broadcast.

## Licensing

Programs are licensed separately, see the top lines of each program source code.
