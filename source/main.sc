#include "base/print.sc"
#include "base/alarm.sc"
#include "builtin.sc"
#include "container/vector.sc"

internal bool game_initialized = false;

internal void game_add_rooms() {
	if (!__room_add("tvscene", "assets/rooms/tvscene.json")) {
		throw "adding the tvscene room failed";
	}
	if (!__room_set_current("tvscene")) {
		throw "setting the game to tvscene failed";
	}
}

internal void game_add_audio() {
	if (!__audio_ogg_init("main_music", "assets/audio/test.ogg")) {
		throw "failed to init assets/audio/test.ogg";
	}
}

void game_frame() {
	alarm_update();
	if (!game_initialized) {
		try {
			game_add_rooms();
			game_add_audio();
			game_initialized = true;
		} catch (string exception) {
			error("Game failed to initialize: " + exception, true);
		}
	}
}
