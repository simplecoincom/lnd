## Building WASM

Build with v1.14 of Go.  Previous versions can build, but initial memory setup for the wasm is too low and browsers will not be able to load it. Go v1.14 made this memory setup more dynamic (my understanding at least) so it just works, where previously we would have had to recompile Go toolchain with a changed memory setup size for wasm.

## Packages

You will have to use forks of the folowwing packages located in the simplecoincom github org, each on the wasm branch. Below is the description of what had to be changed to get it working.

### bbolt

Had to stub out a wasm specific db driver.  Code can be added here to abstract where bbolt db lives. This can be done pretty easily in combination with BrowserFS on the frontend so that bbolt doesn't need to know too much about where the browser is storing things. The frontend demo already uses BrowserFS to allow lnd configuration reading and writing with no changes to the lnd code.

### go-flags

Wasm build was failing because go-flags was attempting to detect terminal column size, and that library is not available to wasm build.  Changes to this library was minimal, just making sure wasm builds used the version that can column sizes hard coded.

### btwwallet

The password/passphrase prompts we breaking the wasm build, so for now I hard coded them.