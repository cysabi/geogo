write a go server file.
have a constant variable for the width of the line as 10 meters. "intersecting" includes if 2 lines slightly overlap because of it's width.

## game state
- colors: str[]
- players: Players[]
  - id:
  - name:
  - team:
  - city:
  - lines: an array of polylines
  - claimed: an array of polygons

SendState just sends all of it to the client on get

## endpoints
- /ping: clients send their lat long to the server, which automatically applies that to the state, calls RecieveLat
- /state?id=: if your player id exists, end all of the game state SendState, otherwise send null
- /join: send player id, name, team, city, the backend adds it to the state

func StateEndpoint()
  - query parameter for the player id
  - send all of the state
  - 

func PingEndpoint()
  - query parameter for the player id
  - get timestamp from request sent
  - if position is already in a shape: return
  - find the most recent point on a polyline or the edge of a shape within 20 meters. (make helper function)
  - if point
    - join polyline to point (make helper function)
      - use openstreetmap routes api to get a line to join (make helper function)
    - if line intersects other line
      - create shape (make helper function)
        - subtract already claimed areas from created shape
      - replace lines state with shapes state. (make helper function)
        - if the intersection point is along the line, the code should create a new polyline which is only the extruding part of the line that's left over after the polygon is created.
      - if new shape intersects with opponent lines.
        - destroy the line and any connected lines. (make helper function)
  - else:
    - create new polyline with one point and a length of 0 at that location.
