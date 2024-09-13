package tablodb

const qryEnableForeignKeys = `PRAGMA foreign_keys = ON;`
const qryCreateSystemInfoTable = `CREATE TABLE systemInfo (
  serverID              TEXT NOT NULL PRIMARY KEY,
  serverName            TEXT NOT NULL,
  privateIP             TEXT NOT NULL,
  dbVer                 INT NOT NULL,
  guideLastUpdated      INT NOT NULL,
  recordingsLastUpdated INT NOT NULL,
  scheduledLastUpdated  INT NOT NULL,
  totalSize             INT,
  freeSize              INT
);`
const qryCreateChannelTable = `CREATE TABLE channel (
  channelID     INT NOT NULL PRIMARY KEY,
  callSign      TEXT NOT NULL,
  major         INT NOT NULL,
  minor         INT NOT NULL,
  network       TEXT
);`
const qryCreateShowTable = `CREATE TABLE show (
  showID          INT NOT NULL PRIMARY KEY,
  parentShowID    INT,
  rule            TEXT,
  channelID       INT,
  keepRecording   TEXT NOT NULL,
  count           INT,
  showType        TEXT NOT NULL,
  title           TEXT NOT NULL,
  descript        TEXT,
  releaseDate     INT,
  origRunTime     INT,
  rating          TEXT,
  stars           INT,
  FOREIGN KEY (parentShowID) REFERENCES show(showID),
  FOREIGN KEY (channelID) REFERENCES channel(channelID)
);`
const qryCreateShowAwardTable = `CREATE TABLE showAward (
  showID          INT NOT NULL,
  won             INT NOT NULL,
  awardName       TEXT NOT NULL,
  awardCategory   TEXT NOT NULL,
  awardYear       INT NOT NULL,
  nominee         TEXT,
  PRIMARY KEY (showID, awardName, awardCategory, awardYear, cast),
  FOREIGN KEY (showID) REFERENCES show(showID)
);`
const qryCreateShowGenreTable = `CREATE TABLE showGenre (
  showID        INT NOT NULL,
  genre         TEXT NOT NULL,
  PRIMARY KEY (showID, genre),
  FOREIGN KEY (showID) REFERENCES show(showID)
);`
const qryCreateShowCastTable = `CREATE TABLE showCast (
  showID        INT NOT NULL,
  cast          TEXT NOT NULL,
  PRIMARY KEY (showID, cast),
  FOREIGN KEY (showID) REFERENCES show(showID)
);`
const qryCreateShowDirectorTable = `CREATE TABLE showDirector (
  show          INT NOT NULL,
  director      TEXT NOT NULL,
  PRIMARY KEY (showID, castID),
  FOREIGN KEY (showID) REFERENCES show(showID)
);`
const qryCreateTeamTable = `CREATE TABLE team (
  teamID        INT NOT NULL PRIMARY KEY,
  team          TEXT NOT NULL
);`
const qryCreateEpisodeTable = `CREATE TABLE episode (
  episodeID       TEXT NOT NULL PRIMARY KEY,
  showID          INT NOT NULL,
  title           TEXT,
  descript        TEXT,
  episode         INT,
  season          TEXT,
  seasonType      TEXT,
  originalAirDate INT,
  homeTeamID      INT,
  FOREIGN KEY (showID) REFERENCES show(showID),
  FOREIGN KEY (homeTeamID) REFERENCES team(teamID)
);`
const qryCreateAiringTable = `CREATE TABLE airing (
  airingID      INT NOT NULL PRIMARY KEY,
  showID        INT NOT NULL,
  airDate       INT NOT NULL,
  duration      INT NOT NULL,
  channelID     INT NOT NULL,
  scheduled     TEXT NOT NULL,
  episodeID     TEXT,
  FOREIGN KEY (showID) REFERENCES show(showID),
  FOREIGN KEY (channelID) REFERENCES channel(channelID),
  FOREIGN KEY (episodeID) REFERENCES episode(episodeID)
);`
const qryCreateEpisodeTeamTable = `CREATE TABLE episodeTeam (
  episodeID     TEXT NOT NULL,
  teamID        INT NOT NULL,
  PRIMARY KEY (episodeID, teamID),
  FOREIGN KEY (episodeID) REFERENCES episode(episodeID),
  FOREIGN KEY (teamID) REFERENCES team(teamID)
);`
const qryCreateRecordingTable = `CREATE TABLE recording (
  recordingID       INT NOT NULL PRIMARY KEY,
  showID            INT NOT NULL,
  airDate           INT NOT NULL,
  airingDuration    INT NOT NULL,
  channelID         INT NOT NULL,
  recordingState    TEXT NOT NULL,
  clean             INT NOT NULL,
  recordingDuration INT NOT NULL,
  recordingSize     INT NOT NULL,
  comSkipState      TEXT NOT NULL,
  episodeID         INT,
  FOREIGN KEY (showID) REFERENCES show(showID),
  FOREIGN KEY (channelID) REFERENCES channel(channelID),
  FOREIGN KEY (episodeID) REFERENCES episode(episodeID)
);`
const qryCreateErrorTable = `CREATE TABLE error (
  errorID           INTEGER PRIMARY KEY,
  recordingID       INT NOT NULL,
  recordingShowID   INT NOT NULL,
  showID            INT,
  episodeID         INT,
  channelID         INT NOT NULL,
  airDate           INT NOT NULL,
  airingDuration    INT NOT NULL,
  recordingDuration INT NOT NULL,
  recordingSize     INT NOT NULL,
  recordingState    TEXT NOT NULL,
  clean             INT NOT NULL,
  comSkipState      TEXT NOT NULL,
  comSkipError      TEXT,
  errorCode         TEXT,
  errorDetails      TEXT,
  errorDescription  TEXT,
);`
