#ifndef BUILTIN_SC
#define BUILTIN_SC

#include "base/bool.sc"

//
// Built-in functionality provided by the interpreter
//

//
// GAME'S STATE MANAGEMENT
//

void __terminate();

//
// PRINTING FUNCTIONALITY
//

void __stdout(auto value);
void __stderr(auto value);

//
// ROOM MANAGEMENT
//

bool __room_add(string name, string filepath);
bool __room_set_current(string name);
bool __room_set_object_pos(string type, int x, int y);
int __room_get_object_x(string type);
int __room_get_object_y(string type);
bool __room_set_object_subsprite_index(string type, int subsprite_index);
int __room_get_object_subsprite_count(string type);
bool __room_object_has_attributes(string type, string attributes[]);

//
// KEYBOARD FUNCTIONALITY
//

bool __key_pressed(string key);
bool __key_just_pressed(string key);
bool __key_just_releazed(string key);

#endif
