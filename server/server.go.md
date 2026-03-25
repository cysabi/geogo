a small server for a worldscale game. this game is a irl version of paper.io, where you capture areas by walking on streets, and completing polygons.

depend on libraries as much as humanly possible. specifically chi, go-geo. i do not need to use s2/s1, as the actual points are being received from

## game state
- lobbies:
  - [code]: Game
      - colors: str[]
      - players: Players[]
        - id:
        - name:
        - team:
        - city:
        - lastPoint: pointer to the latest point
        - lines: an array of polylines
        - claimed: an array of polygons

## endpoints
- /join: send lobby code, player id, name, team, city, the backend adds it to the state
- /ping: clients send a list of lat long to the server, which automatically applies that to the state, calls RecieveLat
- /state: recieves lobby id & player id. all of the game state SendState, otherwise send null

func StateEndpoint()
  - the parameters are
    - the lobby code
    - the player id
  - if lobby cannot be found, return null. this informs the frontend to ask the user to type their lobby code
  - return all of the state for the lobby. the frontend can check if the player id exists or not

func JoinEndpoint()
  - the parameters are
    - the lobby code
    - the player id
    - the team
    - the city
  - if the lobby code is "", then create a new lobby
  - if the player id doesn't exist for this lobby, add them in
  - if they do exist, upsert their name, team, city
  - and the end of it, return all of the state for the lobby

func PingEndpoint()
  - the parameters are
    - the lobby code
    - the player id
    - an array of lat long in ascending order, where the first element in the oldest one, and the last one is the latest one
  - if the player's latest point is within 500m of the oldest point.
    - join the polyline associated to that point to a polyline of 
    - join polyline associated with point
    - if line intersects with other lines by the same player
      - create shape from the polylines
        - subtract already claimed areas from created shape
      - add to player state, and delete the sections of polylines that were used to make the shape, this might require splitting the loop into multiple line segments.
      - if new shape intersects with opponent lines.
        - destroy opponent line and any connected lines.
  - else:
    - create a new polyline using these points
