#include "base/print.sc"
#include "base/alarm.sc"
#include "builtin.sc"
#include "container/vector.sc"

bool game_initialized = false;

void game_add_rooms() {
	if (!__room_add("tvscene", "assets/rooms/tvscene.json")) {
		throw "adding the tvscene room failed";
	}
}

void game_frame() {
	if (!game_initialized) {
		try {
			game_add_rooms();
			game_initialized = true;
		} catch (string exception) {
			error("A room failed to add: " + exception, true);
		}
	}
	alarm_update();
}
