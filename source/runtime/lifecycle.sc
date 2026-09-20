#ifndef RUNTIME_LIFECYCLE_SC
#define RUNTIME_LIFECYCLE_SC

#include "builtin.sc"

//
// Routine Description
//
//		This function terminates the game.
//
void runtime_lifecycle_stop() {
	__stdout("runtime: Stopping the game's lifecycle");
	__terminate();
}

#endif
