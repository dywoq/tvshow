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
bool __room_object_scale(string type, int scale);
int __room_object_get_scale(string type);
bool __room_is_current(string name);
bool __room_add_sprite(string name, string filepath, int x, int y);
bool __room_set_sprite_pos(string name, int x, int y);
int __room_get_sprite_x(string name);
int __room_get_sprite_y(string name);
bool __room_sprite_scale(string name, int scale);
int __room_sprite_get_scale(string name);

//
// KEYBOARD FUNCTIONALITY
//

bool __key_pressed(string key);
bool __key_just_pressed(string key);
bool __key_just_releazed(string key);

//
// AUDIO FUNCTIONALITY
//

bool __audio_ogg_init(string player_name, string filepath);
bool __audio_close(string player_name);
bool __audio_is_playing(string player_name);
bool __audio_rewind(string player_name);
bool __audio_pause(string player_name);
bool __audio_set_volume(string player_name, float volume);
float __audio_get_volume(string player_name);
bool __audio_play(string player_name);

#endif
