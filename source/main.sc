#include "print.sc"

int current_game_frame = 0;

void game_frame() {
	current_game_frame++;
	info("current game frame: " + stringify(current_game_frame));
}
