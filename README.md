# DBState - Discord Badger backed state

This is an alternate state tracker for discord, it's backed by badger and atm the state is not valid between restarts (although that is planned to change for certain obejcts, like message tracking)

Why have a slower, on disk state than a super fast in memory one? Because as your bot grows you will notice that depending on how much state you need to track, it will use more and more memory, in my personal experience the memory usage goes up far more than the cpu and other resources as your bot grows. This is created to solve that.

The objects in the state are encoded using `encoding/gob`.

**Notes**: It's reccomended that you run this with syncevents on, to make sure events aren't being handled out of order

This is still in development, The status is shown below:


**Status:**

The API is not stable currently, it will stabilise as it gets closer to the 1.0 release, see the 1.0 milestone [here](https://github.com/jonas747/dbstate/milestone/1)

## Pros and cons

 - Pro/Con: It's on disk, there is higher latency and cpu usage but it wont eat it up all your memory 
 - Con: Parts are split up, e.g getting a guild dosen't include the members on that guild, that has to be queried seperately

## Benchmarks and performance comparisons to the standard state tracker

TODO (but probably somewhat slower, esp on hdds)