
export type Player = {
  tag: string;
  team: string;
  city: string;
  lastPoint: [number, number] | null;
  trail: GeoJSON.MultiLineString;
  claimed: GeoJSON.MultiPolygon;
};

export type GameState = {
  colors: string[];
  players: Player[];
};

export const SERVER = "192.168.1.171:9090"
