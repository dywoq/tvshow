#include "print.sc"
#include "container/vector.sc"

int current_game_frame = 0;
vector_t vector;

void game_frame() {
	current_game_frame++;
	vector_push(vector, 2);
	info("vector: " + stringify(vector.storage));
}
