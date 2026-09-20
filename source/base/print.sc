#ifndef BASE_PRINT_SC
#define BASE_PRINT_SC

#include "runtime/lifecycle.sc"
#include "builtin.sc"
#include "base/bool.sc"

//
// Routine Description
//
//		This function prints the provided value into the standard out stream.
//		It has the "INFO: " prefix.
//
// Parameters
//
//		v
//
//			The value to print. It is converted into the string.
//
void info(auto v) {
	__stdout("INFO: " + stringify(v));
}

//
// Routine Description
//
//		This function prints the provided value into the standard out stream.
//		It has the "WARNING: " prefix.
//
// Parameters
//
//		v
//
//			The value to print. It is converted into the string.
//
void warn(auto v) {
	__stdout("WARNING: " + stringify(v));
}

//
// Routine Description
//
//		This function prints the provided value into the standard error stream.
//		It has the "WARNING: " prefix.
//
// Parameters
//
//		v
//
//			The value to print. It is converted into the string.
//
//		terminate
//
//			Whether to terminate the program.
//
void error(auto v, bool terminate) {
	__stderr("ERROR: " + stringify(v));
	if (terminate) {
		__stderr("Terminating program due to an error \"" + stringify(v) + "\"");
		runtime_lifecycle_stop();
	}
}

#endif
