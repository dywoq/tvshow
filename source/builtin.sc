#ifndef BUILTIN_SC
#define BUILTIN_SC

#include "base/bool.sc"

//
// Built-in functionality provided by the interpreter
//

void __terminate();
void __stdout(auto value);
void __stderr(auto value);
bool __room_add(string name, string filepath);
bool __room_set_current(string name);
bool __room_set_object_pos(string type, int x, int y);

#endif
