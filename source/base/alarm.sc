#ifndef BASE_ALARM_SC
#define BASE_ALARM_SC

#include "container/vector.sc"
#include "base/print.sc"

//
// Routine Description
//
//		This type alias is a pointer to an alarm's function. It is activated
//		when an alarm's duration is below 0.
//
typedef void (*alarm_func_t)();

//
// Routine Description
//
//		This structure contains the alarm's information.
//
typedef struct alarm {
	int duration;
	alarm_func_t func;
} alarm_t;

vector_t alarms;

//
// Routine Description
//
//		This function schedules the alarm by puhsing a new alarm instance
//		onto the underyling alarms vector.
//
// Parameters
//
//		duration
//
//			Duration of the alarm.
//
//		func
//
/			A pointer to the alarm function.
//
void alarm_schedule(int duration, alarm_func_t func) {
	vector_push(alarms, (alarm_t){ .duration = duration, .func = func });
}

//
// Routine Descrpition
//
//		This function iterates over scheduled alarms and decreases their duration.
//		If the duration of one of alarms is below 0, their function are activated and they are removed
//		from the array.
//
void alarm_update() {
	for (int i = 0; i < alarms.size; i++) {
		alarm_t alarm = alarms.storage[i];
		alarm.duration--;
		if (alarm.duration < 0) {
			alarm.func();
			vector_delete(alarms, i);
		}
	}
}

#endif
