package tablodb

const dbVer = 1

// I'll need this later: UPDATE SQLITE_SEQUENCE SET SEQ=0 WHERE NAME='table_name';

var queries = map[string]string{
	// Create entire database:
	"createDatabase": `-- Turn on foreign key support
PRAGMA foreign_keys = ON;

-- Create systemInfo table
CREATE TABLE systemInfo (
  serverID              TEXT NOT NULL PRIMARY KEY,
  serverName            TEXT NOT NULL,
  privateIP             TEXT NOT NULL,
  dbVer                 INT NOT NULL,
  guideLastUpdated      INT NOT NULL,
  recordingsLastUpdated INT NOT NULL,
  scheduledLastUpdated  INT NOT NULL,
  exportPath            TEXT,
  totalSize             INT,
  freeSize              INT
);

-- Create channel table
CREATE TABLE channel (
  channelID INT NOT NULL PRIMARY KEY,
  callSign  TEXT NOT NULL,
  major     INT NOT NULL,
  minor     INT NOT NULL,
  network   TEXT
);

-- Create show table
CREATE TABLE show (
  showID        INT NOT NULL PRIMARY KEY,
  parentShowID  INT,
  rule          TEXT,
  channelID     INT,
  keepRecording TEXT NOT NULL,
  count         INT,
  showType      TEXT NOT NULL,
  title         TEXT NOT NULL,
  descript      TEXT,
  releaseDate   INT,
  origRunTime   INT,
  rating        TEXT,
  stars         INT,
  FOREIGN KEY (parentShowID) REFERENCES show(showID),
  FOREIGN KEY (channelID) REFERENCES channel(channelID)
);

-- Create showAward table
CREATE TABLE showAward (
  showID        INT NOT NULL,
  won           INT NOT NULL,
  awardName     TEXT NOT NULL,
  awardCategory TEXT NOT NULL,
  awardYear     INT NOT NULL,
  nominee       TEXT,
  PRIMARY KEY (showID, awardName, awardCategory, awardYear, nominee),
  FOREIGN KEY (showID) REFERENCES show(showID)
);

-- Create showGenre table
CREATE TABLE showGenre (
  showID INT NOT NULL,
  genre  TEXT NOT NULL,
  PRIMARY KEY (showID, genre),
  FOREIGN KEY (showID) REFERENCES show(showID)
);

-- Create showCast table
CREATE TABLE showCastMember (
  showID     INT NOT NULL,
  castMember TEXT NOT NULL,
  PRIMARY KEY (showID, castMember),
  FOREIGN KEY (showID) REFERENCES show(showID)
);

-- Create showDirector table
CREATE TABLE showDirector (
  showID   INT NOT NULL,
  director TEXT NOT NULL,
  PRIMARY KEY (showID, director),
  FOREIGN KEY (showID) REFERENCES show(showID)
);

-- Create team table
CREATE TABLE team (
  teamID INT NOT NULL PRIMARY KEY,
  team   TEXT NOT NULL
);

-- Create episode table
CREATE TABLE episode (
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
);

-- Create airing table
CREATE TABLE airing (
  airingID  INT NOT NULL PRIMARY KEY,
  showID    INT NOT NULL,
  airDate   INT NOT NULL,
  duration  INT NOT NULL,
  channelID INT NOT NULL,
  scheduled TEXT NOT NULL,
  episodeID TEXT,
  FOREIGN KEY (showID) REFERENCES show(showID),
  FOREIGN KEY (channelID) REFERENCES channel(channelID),
  FOREIGN KEY (episodeID) REFERENCES episode(episodeID)
);

-- Create episodeTeam table
CREATE TABLE episodeTeam (
  episodeID TEXT NOT NULL,
  teamID    INT NOT NULL,
  PRIMARY KEY (episodeID, teamID),
  FOREIGN KEY (episodeID) REFERENCES episode(episodeID),
  FOREIGN KEY (teamID) REFERENCES team(teamID)
);

-- Create recording table
CREATE TABLE recording (
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
);

-- Create error table
CREATE TABLE error (
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
  errorDescription  TEXT
);

-- Create queue table
CREATE TABLE queue (
  queueID INTEGER PRIMARY KEY,
  action  TEXT NOT NULL,
  details TEXT NOT NULL
);`,
	// Select all records from the queue table
	"selectQueue": `
SELECT
  action,
  details
FROM
  queue
ORDER BY
  queueID ASC;`,
}

var templates = map[string]string{
	// Upsert systemInfo
	"upsertSystemInfo": `
INSERT INTO systemInfo (
  serverID,
  serverName,
  privateIP,
  dbVer,
  guideLastUpdated,
  recordingsLastUpdated,
  scheduledLastUpdated
)
VALUES (
  '%s',
  '%s',
  '%s',
  %d,
  0,
  0,
  0
)
ON CONFLICT DO UPDATE SET
  serverName = '%[2]s',
  privateIP = '%s';`,
	// Insert queue record
	"insertQueue": `
INSERT INTO queue (
  action,
  details
)
VALUES (
  '%s',
  '%s'
);`,
	// Insert queue record with high priority
	"insertQueuePriority": `
INSERT INTO queue (
  queueID,
  action,
  details
)
SELECT
  MIN(queueID) - 1,
  '%s',
  '%s'
FROM queue
`,
	// Upsert channel
	"upsertChannel": `
INSERT INTO channel (
  channelID,
  callSign,
  major,
  minor,
  network
)
VALUES (
  %d,
  '%s',
  %d,
  %d,
  '%s'
)
ON CONFLICT DO UPDATE SET
  callSign = '%[2]s',
  major = %d,
  minor = %d
  network = '%s';`,
	// Upsert show
	"upsertShow": `
INSERT INTO show (
  showID,
  showType,
  rule,
  channelID,
  keepRecording,
  count,
  title,
  descript,
  releaseDate,
  origRunTime,
  rating,
  stars
)
VALUES (
  %d,
  '%s',
  '%s',
  %d,
  '%s',
  %d,
  '%s',
  '%s',
  %d,
  %d,
  '%s',
  %d
)
ON CONFLICT DO UPDATE SET
  rule = '%[3]s',
  channelID = %d,
  keepRecording = '%s',
  count = %d,
  title = '%s',
  descript = '%s',
  releaseDate = %d,
  origRunTime = %d,
  rating = '%s',
  stars = %d;`,
	// Insert showGenre
	"insertShowGenre": `
INSERT INTO showGenre (
  showID,
  genre
)
VALUES (
  %d,
  '%s'
)
ON CONFLICT DO NOTHING;`,
	// Insert showCast
	"insertShowCastMember": `
INSERT INTO showCastMember (
  showID,
  castMember
)
VALUES (
  %d,
  '%s'
)
ON CONFLICT DO NOTHING;`,
	// Upsert showAward
	"upsertShowAward": `
INSERT INTO showAward (
  showID,
  won,
  awardName,
  awardCategory,
  awardYear,
  nominee
)
VALUES (
  %d,
  %d,
  '%s',
  '%s',
  %d,
  '%s'
)
ON CONFICT DO UPDATE SET
  won = %[2]d;`,
	// Insert showDirector
	"insertShowDirector": `
INSERT INTO showDirector (
  showID,
  director
)
VALUES (
  %d,
  '%s'
)
ON CONFLICT DO NOTHING;`,
}
