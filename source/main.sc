#include "base/print.sc"
#include "base/alarm.sc"
#include "builtin.sc"
#include "container/vector.sc"

bool game_initialized = false;

void game_frame() {
	if (!game_initialized) {
		if (!__room_add("rm_startup", "./assets/room/startup.json")) {
			error("failed to load rm_startup file", true);
			return;
		}
		game_initialized = true;
	}
	alarm_update();
}
