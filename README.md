# DBState - Discord Badger backed state

This is an alternate state tracker for discord, it's backed by badger and atm the state is not valid between restarts (although that is planned to change for certain obejcts, like message tracking)

Why have a slower, on disk state than a super fast in memory one? Because as your bot grows you will notice that depending on how much state you need to track, it will use more and more memory, in my personal experience the memory usage goes up far more than the cpu and other resources as your bot grows. This is created to solve that.

The objects in the state are encoded using `encoding/gob`.

*Requirements*: Only supports sessions with SyncEvents enabled, this makes everything a lot more simpler for me.

This is still in development, The status is shown below:

*Status:*

 - [x] Guild tracking and acessors/iterators
 - [x] Member tracking and acessors/iterators 
 - [ ] Presence tracking and acessors/iterators 
 - [x] Channel tracking and acessors (channels are currently both tracked on the parent guild and in a global directoryunder the ``channels:` prefix)
 - [ ] Role tracking and acessors (should roles be tracked on guild? probably the simplest solution, although it can get quite big with a lot of channels and roles)
 - [ ] Message tracking and accessors, aswell as a TTL for messages (this TTL feature will hit badger soon apperently)
 - [ ] Voice State tracking and accessors
 - [ ] Emoji State tracking and accessors (Put on guild object?)

