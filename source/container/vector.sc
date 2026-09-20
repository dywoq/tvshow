#ifndef CONTAINER_VECTOR_SC
#define CONTAINER_VECTOR_SC

typedef struct vector {
	auto storage[];
	unsigned int size;
} vector_t;

//
// Exceptions
//

#define VECTOR_EXC_OUT_OF_BOUNDS(given_index, size) "out of bounds (given index: " + stringify(given_index) + ", size: " + stringify(size) + ")"
#define VECTOR_EXC_EMPTY "empty vector"

//
// Routine Description
//
//		Pushes the provided element onto the vector.
//
// Parameters
//
//		v
//
//			The vector instance.
//
//		element
//
//			The element to push.
//
void vector_push(vector_t v, auto element) {
	v.storage[v.size] = element;
	v.size++;
}

//
// Routine Description
//
//		Removes the latest element from the vector.
//
// Parameters
//
//		v
//
//			The vector instance.
//
// Exceptions
//
//		VECTOR_EXC_EMPTY
//
//			The vector is empty.
//
void vector_pop(vector_t v) {
	if (v.size == 0) {
		throw VECTOR_EXC_EMPTY;
	}
	v.storage[v.size-1] = 0;
	v.size--;
}

//
// Routine Description
//
//		Retrieves an element at specified index in the vector.
//
// Parameters
//
//		v
//
//			The vector instance.
//
//		index
//
//			The index of the element.
//
// Exceptions
//
//		VECTOR_EXC_EMPTY
//
//			The vector is empty.
//
//		VECTOR_EXC_OUT_OF_BOUNDS
//
//			The provided index >= the vector's size.
//
auto vector_at(vector_t v, int index) {
	if (v.size == 0) {
		throw VECTOR_EXC_EMPTY;
	}
	if (index >= v.size) {
		throw VECTOR_EXC_OUT_OF_BOUNDS(index, v.size);
	}
	return v.storage[index];
}

//
// Routine Description
//
//		This function deletes an element at specified index in the vector
//		and shifts elements. It decreases the vector's size.
//
// Parameters
//
//		v
//
//			The vector instance.
//
//		index
//
//			The index of the element.
//
// Exceptions
//
//		VECTOR_EXC_EMPTY
//
//			The vector is empty.
//
//		VECTOR_EXC_OUT_OF_BOUNDS
//
//			The provided index >= the vector's size.
//
void vector_delete(vector_t v, int index) {
	if (v.size == 0) {
		throw VECTOR_EXC_EMPTY;
	}
	if (index >= v.size) {
		throw VECTOR_EXC_OUT_OF_BOUNDS(index, v.size);
	}

	for (int i = index; i < v.size - 1; i++) {
		v.storage[i] = v.storage[i + 1];
	}
	v.size--;
}

#endif
