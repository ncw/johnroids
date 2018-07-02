This is JohnRoids.  A simple but addictive game whose sole purpose is
to kill John.  Anyone who knows the John I am talking about will
certainly understand the attraction of this.

I wrote this game originally in 1991 on my Acorn Archimedes to annoy
John when I was bored one rainy afternoon.  John came round and
instead of being annoyed played the game for hours and hours - there
is obviously some kind of perverse thrill in shooting oneself!

Back in 1991 digitised pictures were still something of a rarity.  I
made the one of John from a camcorder video and an extremely hacky
black and white video digitiser which I built myself.  Note there are
actually two images of John - one looking happy and the other not!

The original was written entirely in ARM assembler to make it go fast
enough.  However I thought the program deserved not to whither away so
I ported it to C and SDL and made it compile on Windows and Linux so
many more hours can be wasted shooting at John.

When you run the game you'll notice the playing area is rather small
(320x256 pixels).  I'm afraid this was all that was available in 1991!
I tried making the playing area bigger but the game lost some of its
visceral thrill, so you are left with a small screen.

This is an entirely faithful conversion of the original except for the
fact that I haven't put the sound back in yet.  The original samples
are in a very strange format I haven't managed to translate.

I've included the source code in the archive and the Makefile - hack
away by all means!

To run :-

  Windows:  double click on roids.exe
  Linux:    run roids in the usual fashion

The instructions are printed on the screen to start, but to summarise
they are

  Z      - Rotate left
  X      - Rotate right
  Shift  - Fire thruster
  Return - Fire gun
  Space  - To start the game
  Escape - to end the game

My best score is just over 50,000 - things get very hectic as you go
through the levels.

Enjoy

Nick Craig-Wood
nick@craig-wood.com
2001-09-14
