# DBState - Discord Badger backed state

This is an alternate state tracker for discord, it's backed by badger and atm the state is not valid between restarts (although that is planned to change for certain obejcts, like message tracking)

Why have a slower, on disk state than a super fast in memory one? Because as your bot grows you will notice that depending on how much state you need to track, it will use more and more memory, in my personal experience the memory usage goes up far more than the cpu and other resources as your bot grows. This is created to solve that.

The objects in the state are encoded using `encoding/gob`.

**Notes**: It's reccomended that you run this with syncevents on, to make sure events aren't being handled out of order

This is still in development, The status is shown below:


**Status:**

 - [x] Guild tracking and acessors/iterators
 - [x] Member tracking and acessors/iterators 
 - [ ] Presence tracking and acessors/iterators 
 - [x] Channel tracking and acessors (channels are currently both tracked on the parent guild and in a global directory under the ``channels:` prefix)
 - [x] Role tracking and acessors (should roles be tracked on guild? probably the simplest solution, although it can get quite big with a lot of channels and roles)
 - [ ] Message tracking and accessors, aswell as a TTL for messages 
 - [ ] Voice State tracking and accessors 
 - [ ] Emoji State tracking and accessors (Put on guild object?)

## Pros and cons

 - Pro/Con: It's on disk, there is higher latency and cpu usage but it wont eat it up all your memory 
 - Con: Parts are split up, e.g getting a guild dosen't include the members on that guild, that has to be queried seperately

## Benchmarks and performance comparisons to the standard state tracker

TODO (but probably somewhat slower, esp on hdds)