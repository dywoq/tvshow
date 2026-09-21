#ifndef AUDIO_MANAGER_SC
#define AUDIO_MANAGER_SC

#include "base/bool.sc"
#include "container/vector.sc"
#include "builtin.sc"

typedef struct audio_player_info {
	bool infinite;
	string filepath;
	string name;
} audio_player_info_t;

internal vector_t audio_players_info_;

//
// Routine Description
//
//		Initializes an audio player and plays the provided audio.
//		Throws an exception if it failed to initialize the player
//		or play the audio.
//
// Parameters
//
//		filepath
//
//			Path to the audio.
//
//		infinite
//
//			Whether to play the audio infinitely.
//
void audio_play(string filepath, bool infinite) {
	string player_name = stringify(audio_players_info_.size);
	if (!__audio_ogg_init(player_name, filepath)) {
		throw "failed to initialize audio file " + filepath;
	}
	if (!__audio_play(player_name)) {
		throw "failed to play audio file " + filepath;
	}
	vector_push(audio_players_info_, (audio_player_info_t){
		.infinite = infinite,
		.filepath = filepath,
		.name = player_name,
	});
}

//
// Routine Description
//
//		Closes an audio player, deleting it from the list.
//		Throws an exception if it failed to find an audio player
//		having the provided filepath.
//
// Parameters
//
//		filepath
//
//			Path to the audio.
//
void audio_close(string filepath) {
	if (audio_players_info_.size == 0) {
		return;
	}
	for (int i = 0; i < audio_players_info_.size; i++) {
		audio_player_info_t info = audio_players_info_.storage[i];
		if (info.filepath == filepath) {
			if (!__audio_close(info.name)) {
				throw "failed to close audio " + filepath;
			}
			vector_delete(audio_players_info_, i);
			return;
		}
	}
	throw "failed to find audio " + filepath;
}

//
// Routine Description
//
//		Iterates over underlying audio players. If one of them stops playing
//		and it is intended to play infinitely, the function rewinds it to the start
//		and plays it again.
//
void audio_update() {
	if (audio_players_info_.size == 0) {
		return;
	}
	for (int i = 0; i < audio_players_info_.size; i++) {
		audio_player_info_t info = audio_players_info_.storage[i];
		if (!__audio_is_playing(info.name)) {
			if (info.infinite) {
				__audio_rewind(info.name);
				__audio_play(info.name);
			}
		}
	}
}

//
// Routine Description
//
//		Checks whether the audio player, which has the provided filepath,
//		plays.
//
bool audio_is_playing(string filepath) {
	if (audio_players_info_.size == 0) {
		return false;
	}
	for (int i = 0; i < audio_players_info_.size; i++) {
		audio_player_info_t info = audio_players_info_.storage[i];
		if (info.filepath == filepath) {
			return __audio_is_playing(info.name);
		}
	}
	return false;
}

//
// Routine Description
//
//		Rewinds the audio player, which has the provided filepath,
//		to the start of its stream.
//
//		Throws an exception if it failed to find an audio player
//		having the provided filepath.
//
void audio_rewind(string filepath) {
	if (audio_players_info_.size == 0) {
		throw "no audio playing";
	}
	for (int i = 0; i < audio_players_info_.size; i++) {
		audio_player_info_t info = audio_players_info_.storage[i];
		if (info.filepath == filepath) {
			if (!__audio_rewind(info.name)) {
				throw "failed to rewind audio " + info.filepath;
			}
			return;
		}
	}
	throw "failed to find audio " + filepath;
}

//
// Routine Description
//
//		Pauses the audio player.
//
//		Throws an exception if it failed to find an audio player
//		having the provided filepath.
//
void audio_pause(string filepath) {
	if (audio_players_info_.size == 0) {
		throw "no audio playing";
	}
	for (int i = 0; i < audio_players_info_.size; i++) {
		audio_player_info_t info = audio_players_info_.storage[i];
		if (info.filepath == filepath) {
			if (!__audio_pause(info.name)) {
				throw "failed to pause audio " + info.filepath;
			}
			return;
		}
	}
	throw "failed to find audio " + filepath;
}

//
// Routine Description
//
//		Modifies the volume in the audio player.
//
//		Throws an exception if it failed to find an audio player
//		having the provided filepath.
//
void audio_set_volume(string filepath, float volume) {
	if (audio_players_info_.size == 0) {
		throw "no audio playing";
	}
	for (int i = 0; i < audio_players_info_.size; i++) {
		audio_player_info_t info = audio_players_info_.storage[i];
		if (info.filepath == filepath) {
			if (!__audio_set_volume(info.name, volume)) {
				throw "failed to set volume in audio " + info.filepath;
			}
			return;
		}
	}
	throw "failed to find audio " + filepath;
}

//
// Routine Description
//
//		Provides the current volume of the audio player.
//
//		Throws an exception if it failed to find an audio player
//		having the provided filepath.
//
float audio_get_volume(string filepath) {
	if (audio_players_info_.size == 0) {
		throw "no audio playing";
	}
	for (int i = 0; i < audio_players_info_.size; i++) {
		audio_player_info_t info = audio_players_info_.storage[i];
		if (info.filepath == filepath) {
			float volume = __audio_get_volume(info.name);
			if (volume == -1) {
				throw "unable to get volume of audio " + filepath;
			}
			return volume;
		}
	}
	throw "failed to find audio " + filepath;
}

#endif
