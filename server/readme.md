# Cityblock API Specification

Base URL: `http://localhost:9090`

All coordinates are `[lng, lat]` (GeoJSON order).

---

## GET /state

Poll this endpoint to get the current game state for a lobby.

### Query Parameters

| Param   | Type   | Required | Description       |
|---------|--------|----------|-------------------|
| `lobby` | string | yes      | The lobby code    |

### Response

Returns `null` if the lobby doesn't exist — this signals the frontend to prompt the user for a lobby code.

Otherwise returns the full game state:

```json
{
  "colors": ["#ff0000", "#00ff00"],
  "players": [
    {
      "id": "player-uuid",
      "name": "Alice",
      "team": "red",
      "city": "nyc",
      "lastPoint": [lng, lat],
      "trail": [
        [
          [[lng, lat], [lng, lat], ...],
          [[lng, lat], [lng, lat], ...]
        ]
      ],
      "claimed": [
        [
          [[lng, lat], [lng, lat], ...],
          [[lng, lat], [lng, lat], ...]
        ]
      ]
    }
  ]
}
```

#### Player fields

| Field       | Type                   | Description                                                        |
|-------------|------------------------|--------------------------------------------------------------------|
| `id`        | string                 | Unique player identifier                                           |
| `name`      | string                 | Display name                                                       |
| `team`      | string                 | Team identifier                                                    |
| `city`      | string                 | City the player is in (`"nyc"`, `"london"`, `"shanghai"`)          |
| `lastPoint` | `[lng, lat]` or `null` | The most recent coordinate (tip of the active trail)               |
| `trail`     | polygon array or `null`| Buffered road segments the player has walked. Same format as `claimed` — array of polygons, each polygon is an array of rings. |
| `claimed`   | polygon array or `null`| Enclosed territory. Array of polygons, each polygon is an array of rings (first ring = exterior, rest = holes). Each ring is an array of `[lng, lat]` coords. |

### Frontend flow

1. On launch, call `GET /state?lobby=<code>`.
2. If `null`, show a join/create lobby screen.
3. If a game is returned, check if your player ID exists in `players`. If not, call `/join`.
4. Poll `/state` on an interval to render updated trail and claimed areas.

---

## POST /join

Join or create a lobby. Also used to update player info.

### Request Body (JSON)

| Field    | Type   | Required | Description                                            |
|----------|--------|----------|--------------------------------------------------------|
| `lobby`  | string | no       | Lobby code. If `""` or omitted, a new lobby is created |
| `player` | string | yes      | Unique player ID (e.g. a UUID)                         |
| `name`   | string | no       | Display name                                           |
| `team`   | string | no       | Team identifier                                        |
| `city`   | string | no       | City (`"nyc"`, `"london"`, `"shanghai"`)               |

### Response

```json
{
  "lobby": "a1b2c3d4",
  "game": { ... }
}
```

- `lobby` — the lobby code (important when creating a new lobby, since the server generates the code)
- `game` — full game state (same shape as `/state` response)

### Errors

| Status | Condition                    |
|--------|------------------------------|
| 400    | Missing `player`             |
| 404    | Lobby code provided but not found |

### Frontend flow

1. **Create a lobby**: `POST /join` with `lobby: ""`. Save the returned `lobby` code and share it with other players.
2. **Join an existing lobby**: `POST /join` with the lobby code and your player info.
3. **Update your info**: Call `/join` again with the same `lobby` and `player` — name, team, and city are upserted.

---

## POST /ping

Send location updates from the client. The server snaps points to roads (via OSRM), buffers them into a trail polygon, detects enclosed areas, and handles territory claiming.

### Request Body (JSON)

| Field    | Type             | Required | Description                                                  |
|----------|------------------|----------|--------------------------------------------------------------|
| `lobby`  | string           | yes      | Lobby code                                                   |
| `player` | string           | yes      | Player ID                                                    |
| `points` | `[lng, lat][]`   | yes      | Array of coordinates in ascending time order (oldest first)  |

### Response

Returns `null` on success.

### Errors

| Status | Condition                    |
|--------|------------------------------|
| 400    | Missing fields or empty points |
| 404    | Lobby or player not found    |
| 500    | Road-snapping or geometry error |

### Frontend flow

1. Collect GPS coordinates from the device at a regular interval.
2. Batch them and send to `/ping` periodically (e.g. every few seconds).
3. Points should be `[lng, lat]` — longitude first, latitude second (GeoJSON order).
4. The server handles everything else: snapping to roads, building the trail, detecting enclosed areas, and claiming territory.

### Server behavior details

- If the player's `lastPoint` is within **1000m** of the first point in the batch, the segment continues from the last position.
- Otherwise, a **new segment** is started (requires 2+ points).
- All points are snapped to the nearest road via OSRM, then buffered into a polygon strip and unioned into the player's trail.
- When the trail forms a closed loop (creating a hole in the trail polygon), the enclosed area is automatically claimed as territory.
- Claimed territory subtracts from opponents' claimed areas and trail.

---

## Typical Client Lifecycle

```
1.  GET  /state?lobby=<saved_code>
    -> null? Show lobby screen
    -> game? Check if player exists

2.  POST /join  { lobby: "", player: "uuid", name: "Alice", team: "red", city: "nyc" }
    -> save returned lobby code
    -> share code with friends

3.  Loop:
      POST /ping  { lobby, player, points: [[lng,lat], ...] }
      GET  /state?lobby=<code>
      -> render all players' trail and claimed areas on the map
```
