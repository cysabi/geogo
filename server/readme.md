# Territory Game — API Reference

Base URL: `http://<host>:8080`

---

## GET `/state`

Returns the full game state as JSON. No parameters required.

### Response

```json
{
  "colors": ["red", "blue", "green", "yellow"],
  "players": [
    {
      "id": "p1",
      "team": "red",
      "city": "New York",
      "lines": [
        {
          "points": [
            { "lat": 40.7128, "lng": -74.006, "timestamp": "2026-03-23T14:00:00Z" },
            { "lat": 40.7130, "lng": -74.005, "timestamp": "2026-03-23T14:00:05Z" }
          ]
        }
      ],
      "claimed": [
        {
          "vertices": [
            { "lat": 40.7128, "lng": -74.006 },
            { "lat": 40.7132, "lng": -74.005 },
            { "lat": 40.7130, "lng": -74.007 }
          ]
        }
      ]
    }
  ]
}
```

### Field Reference

| Field | Type | Description |
|---|---|---|
| `colors` | `string[]` | Available team colors. |
| `players` | `Player[]` | Every player currently in the game. |
| `players[].id` | `string` | Unique player identifier. |
| `players[].team` | `string` | The team/color this player belongs to. |
| `players[].city` | `string` | The city the player is playing in. |
| `players[].lines` | `PlayerLine[]` | Active polylines the player is drawing. |
| `players[].lines[].points` | `TimedPoint[]` | Ordered vertices of the polyline. |
| `players[].claimed` | `ClaimedArea[]` | Closed polygons the player has captured. |
| `players[].claimed[].vertices` | `LatLng[]` | Ordered vertices of the polygon boundary. |

---

## GET `/ping`

Send the player's current position. The server applies it to the game state and returns the updated state.

### Query Parameters

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | `string` | **Yes** | The player's unique ID. |
| `lat` | `float` | **Yes** | Latitude in decimal degrees (WGS 84). |
| `lng` | `float` | **Yes** | Longitude in decimal degrees (WGS 84). |
| `ts` | `int` | No | Unix timestamp in **milliseconds** when the position was recorded on the client. Defaults to server time if omitted. |

### Example Request

```
GET /ping?id=p1&lat=40.71280&lng=-74.00600&ts=1774556400000
```

### Response

On success, the response body is the full game state (identical schema to `GET /state`), reflecting the world **after** the ping has been processed.

### Error Responses

| Status | Body | Cause |
|---|---|---|
| `400` | `missing id` | The `id` query parameter was not provided. |
| `400` | `bad lat` | The `lat` parameter is missing or not a valid float. |
| `400` | `bad lng` | The `lng` parameter is missing or not a valid float. |
| `500` | `unknown player <id>` | No player with that ID exists in the game state. |

---

## Example: cURL

```bash
# Fetch current state
curl "http://localhost:8080/state"

# Send a position ping
curl "http://localhost:8080/ping?id=p1&lat=40.71280&lng=-74.00600&ts=1774556400000"
```

## Example: JavaScript (fetch)

```js
const BASE = "http://localhost:8080";

async function getState() {
  const res = await fetch(`${BASE}/state`);
  return res.json();
}

async function ping(playerId, lat, lng) {
  const ts = Date.now();
  const res = await fetch(
    `${BASE}/ping?id=${playerId}&lat=${lat}&lng=${lng}&ts=${ts}`
  );
  return res.json();
}
```
