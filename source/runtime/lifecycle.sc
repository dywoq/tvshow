#ifndef RUNTIME_LIFECYCLE_SC
#define RUNTIME_LIFECYCLE_SC

#include "builtin.sc"

//
// Routine Description
//
//
//		This function terminates the current program task and reports it into the standard stream.
//		It does not return. The function does not stop the game's execution unless an end-user explicitly
//		presses "yes".
//
void runtime_lifecycle_stop() {
	__stdout("runtime: Stopping the program's lifecycle");
	__terminate();
}

#endif
