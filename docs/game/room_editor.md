# Room Editor

## Overview

The room editor is a cross-platform graphical application that supports editing rooms and exporting them into JSON format.
It is developed in Golang and is in the `game/editor` package of the repository.

### JSON File structure

| **Parameter**    | **Type**   | **Description**                                        |
| ---------------- | ---------- | ------------------------------------------------------ |
| `width`          | `integer`  | Room width.                                            |
| `height`         | `integer`  | Room height.                                           |
| `object_layers`  | `struct[]` | Contains room layers that are from the object group.   |
| `tileset_layers` | `struct[]` | Contains room layers that are from the tile set group. |

### Layer

#### Groups

| **Group Name** | **Purpose**                                                                                            |
| -------------- | ------------------------------------------------------------------------------------------------------ |
| Object Group   | Consists of room objects.                                                                              |
| Tile set Group | Needed to draw tiles, which have identical resolution (e.g. 8x8, 16x16, 32x32, 64x64 etc.), in a room. |

#### Object

Object's information consists of the following parameters that are exported into a JSON file:

| **Parameter** | **Type**                     | **Description**                         |
| ------------- | ---------------------------- | --------------------------------------- |
| `type`        | `string`                     | The user-defined object type.           |
| `attributes`  | `string[]`                   | The user-defined object attributes.     |
| `coordinates` | `{ x: integer, y: integer }` | The object's coordinates within a room. |

#### Tile set

Tile set's information consists of the following parameters that are exported into a JSON file:

| **Parameter**  | **Type**   | **Description**                                                                                      |
| -------------- | ---------- | ---------------------------------------------------------------------------------------------------- |
| `tileset_path` | `string`   | User-defined relative path to a tileset asset. If it is not found, the editor explicitly reports it. |
| `attributes`   | `string[]` | The user-defined tile set attributes.                                                                |
| `tile_width`   | `integer`  | A width of a tile.                                                                                   |
| `tile_height`  | `integer`  | A height of a tile.                                                                                  |
| `tiles`        | `struct[]` | The user-defined tiles, set within the room editor.                                                  |

A tile, which is put into the `tiles` array of a tile set layer, consists of the following information:

| **Parameter** | **Type** | **Description**                      |
| ------------- | -------- | ------------------------------------ |
| `index`       | `string` | Tile index within the tileset asset. |
