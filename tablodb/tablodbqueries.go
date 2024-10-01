package tablodb

const dbVer = 1

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
  defaultExportPath     TEXT,
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
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE
);

-- Create showGenre table
CREATE TABLE showGenre (
  showID INT NOT NULL,
  genre  TEXT NOT NULL,
  PRIMARY KEY (showID, genre),
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE
);

-- Create showCast table
CREATE TABLE showCastMember (
  showID     INT NOT NULL,
  castMember TEXT NOT NULL,
  PRIMARY KEY (showID, castMember),
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE
);

-- Create showDirector table
CREATE TABLE showDirector (
  showID   INT NOT NULL,
  director TEXT NOT NULL,
  PRIMARY KEY (showID, director),
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE
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
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE,
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
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE,
  FOREIGN KEY (channelID) REFERENCES channel(channelID) ON DELETE CASCADE,
  FOREIGN KEY (episodeID) REFERENCES episode(episodeID) ON DELETE CASCADE
);

-- Create episodeTeam table
CREATE TABLE episodeTeam (
  episodeID TEXT NOT NULL,
  teamID    INT NOT NULL,
  PRIMARY KEY (episodeID, teamID),
  FOREIGN KEY (episodeID) REFERENCES episode(episodeID) ON DELETE CASCADE,
  FOREIGN KEY (teamID) REFERENCES team(teamID) ON DELETE CASCADE
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
  episodeID         TEXT,
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE,
  FOREIGN KEY (channelID) REFERENCES channel(channelID) ON DELETE CASCADE,
  FOREIGN KEY (episodeID) REFERENCES episode(episodeID) ON DELETE CASCADE
);

-- Create error table
CREATE TABLE error (
  errorID           INTEGER PRIMARY KEY,
  recordingID       INT NOT NULL,
  showID            INT NOT NULL,
  episodeID         TEXT,
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
  queueID    INTEGER PRIMARY KEY,
  action     TEXT NOT NULL,
  details    TEXT NOT NULL,
  exportPath TEXT NOT NULL
);

-- Create priority table
CREATE TABLE showPriority (
  showID   INT NOT NULL,
  priority INT UNIQUE,
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE
);

-- Create scheduleConflicts table
CREATE TABLE scheduleConflicts (
  airingID INT NOT NULL PRIMARY KEY,
  showID   INT NOT NULL,
  airDate  INT NOT NULL,
  endDate  INT NOT NULL,
  FOREIGN KEY (airingID) REFERENCES airing(airingID) ON DELETE CASCADE,
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE
);

-- Create exported table
CREATE TABLE exported (
  fullPath TEXT NOT NULL PRIMARY KEY
);

-- Create filter table
CREATE TABLE showFilter (
  showID INT NOT NULL PRIMARY KEY,
  ignore INT,
  FOREIGN KEY (showID) REFERENCES show(showID) ON DELETE CASCADE
);`,
	// Select all records from the queue table
	"selectQueue": `
SELECT
  queueID,
  action,
  details,
  exportPath
FROM
  queue
ORDER BY
  queueID ASC;`,
	// Get LastUpdated values from systemInfo
	"getLastUpdated": `
SELECT
  guideLastUpdated,
  scheduledLastUpdated,
  recordingsLastUpdated
FROM
  systemInfo;`,
	// Get defaultExportPath from systemInfo
	"getDefaultExportPath": `
SELECT
  COALESCE(defaultExportPath, '') as defaultExportPath
FROM
  systemInfo;`,
	// Get dbVer from systemInfo
	"getDBVer": `
SELECT
  dbVer
FROM
  systemInfo;`,
	// Get conflicts from airing
	"selectConflicts": `
SELECT
  airingID,
  showID,
  airDate,
  duration
FROM
  airing
WHERE
  scheduled = 'conflict';`,
	// Get scheduled from airing
	"selectScheduled": `
SELECT
  airingID,
  showID,
  airDate,
  duration
FROM
  airing
WHERE
  scheduled = 'scheduled';`,
	// Delete all values from conflicts
	"deleteConflicts": `
DELETE FROM scheduleConflicts;`,
	// Select exported
	"selectExported": `
SELECT
  fullPath
FROM
  exported;`,
	// Select all scheduled airings
	"selectScheduledAirings": `
SELECT
  a.airingID,
  s.showType,
  s.title AS showTitle,
  COALESCE(e.season, '') AS season,
  COALESCE(e.episode, 0) AS episode,
  a.airDate,
  COALESCE(e.title, '') AS episodeTitle,
  COALESCE(s.releaseDate, 0) as releaseDate
FROM
  airing AS a
  INNER JOIN show AS s ON a.showID = s.showID
  LEFT JOIN episode AS e ON a.episodeID = e.episodeID
WHERE
  scheduled IN ('scheduled','conflict');`,
	// update scheduled airings to none
	"updateAiringScheduledToNone": `
UPDATE airing
SET scheduled = 'none'
WHERE scheduled in ('conflict','scheduled');`,
	// select conflicted shows & priority
	"selectPriorityConflicts": `
SELECT
  sc.airingID,
  s.showType,
  COALESCE(sp.priority, -1) AS priority,
  sc.airDate,
  sc.endDate
FROM
  scheduleConflicts sc
  INNER JOIN show s ON sc.showID = s.showID
  LEFT JOIN showPriority sp ON sc.showID = sp.showID
ORDER BY
  sc.airDate,
  sc.endDate,
  COALESCE(sp.priority, 0),
  sc.airingID;`,
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
  serverName = excluded.serverName,
  privateIP = excluded.privateIP;`,
	// Insert queue record
	"insertQueue": `
INSERT INTO queue (
  action,
  details,
  exportPath
)
VALUES (
  '%s',
  '%s',
  '%s'
);`,
	// Insert queue record with high priority
	"insertQueuePriority": `
INSERT INTO queue (
  queueID,
  action,
  details,
  exportPath
)
SELECT
  MIN(queueID) - 1,
  '%s',
  '%s',
  '%s'
FROM queue;`,
	// Upsert channel
	"upsertChannel": `
INSERT INTO channel (
  channelID,
  callSign,
  major,
  minor,
  network
)
VALUES
%s
ON CONFLICT DO UPDATE SET
  callSign = excluded.callSign,
  major = excluded.major,
  minor = excluded.minor,
  network = excluded.network;`,
	// Upsert show
	"upsertShow": `
INSERT INTO show (
  showID,
  parentShowID,
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
VALUES
%s
ON CONFLICT DO UPDATE SET
  rule = excluded.rule,
  channelID = excluded.channelID,
  keepRecording = excluded.keepRecording,
  count = excluded.count,
  title = excluded.title,
  descript = excluded.descript,
  releaseDate = excluded.releaseDate,
  origRunTime = excluded.origRunTime,
  rating = excluded.rating,
  stars = excluded.stars;`,
	// Insert showGenre
	"insertShowGenre": `
INSERT INTO showGenre (
  showID,
  genre
)
VALUES
  %s
ON CONFLICT DO NOTHING;`,
	// Insert showCast
	"insertShowCastMember": `
INSERT INTO showCastMember (
  showID,
  castMember
)
VALUES
  %s
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
VALUES
  %s
ON CONFLICT DO UPDATE SET
  won = excluded.won;`,
	// Insert showDirector
	"insertShowDirector": `
INSERT INTO showDirector (
  showID,
  director
)
VALUES
  %s
ON CONFLICT DO NOTHING;`,
	// Delete queue record
	"deleteQueueRecord": `
DELETE FROM queue
WHERE queueID = %d;`,
	// Upsert team
	"upsertTeam": `
INSERT INTO team (
  teamID,
  team
)
VALUES
%s
ON CONFLICT DO UPDATE SET
  team = excluded.team;`,
	// Upsert episode
	"upsertEpisode": `
INSERT INTO episode (
  episodeID,
  showID,
  title,
  descript,
  episode,
  season,
  seasonType,
  originalAirDate,
  homeTeamID
)
VALUES
%s
ON CONFLICT DO UPDATE SET
  showID = excluded.showID,
  title = excluded.title,
  descript = excluded.descript,
  episode = excluded.episode,
  season = excluded.season,
  seasonType = excluded.seasonType,
  originalAirDate = excluded.originalAirDate,
  homeTeamID = excluded.homeTeamID;`,
	// Insert episodeTeam
	"insertEpisodeTeam": `
INSERT INTO episodeTeam (
  episodeID,
  teamID
)
VALUES
%s
ON CONFLICT DO NOTHING;`,
	// Upsert airing
	"upsertAiring": `
INSERT INTO airing (
  airingID,
  showID,
  airDate,
  duration,
  channelID,
  scheduled,
  episodeID
)
VALUES
%s
ON CONFLICT DO UPDATE SET
  showID = excluded.showID,
  airDate = excluded.airDate,
  duration = excluded.duration,
  channelID = excluded.channelID,
  scheduled = excluded.scheduled,
  episodeID = excluded.episodeID;`,
	// Update guideLastUpdated in systemInfo
	"updateGuideLastUpdated": `
UPDATE systemInfo
SET guideLastUpdated = %d`,
	// Update scheduledLastUpdated in systemInfo
	"updateScheduledLastUpdated": `
UPDATE systemInfo
SET scheduledLastUpdated = %d`,
	// Update scheduledLastUpdated in systemInfo
	"updateRecordingsLastUpdated": `
UPDATE systemInfo
SET recordingsLastUpdated = %d`,
	// Insert error
	"upsertRecording": `
INSERT INTO recording (
  recordingID,
  showID,
  airDate,
  airingDuration,
  channelID,
  recordingState,
  clean,
  recordingDuration,
  recordingSize,
  comSkipState,
  episodeID
)
VALUES
%s
ON CONFLICT DO UPDATE SET
  showID = excluded.showID,
  airDate = excluded.airDate,
  airingDuration = excluded.airingDuration,
  channelID = excluded.channelID,
  recordingState = excluded.recordingState,
  clean = excluded.clean,
  recordingDuration = excluded.recordingDuration,
  recordingSize = excluded.recordingSize,
  comSkipState = excluded.comSkipState,
  episodeID = excluded.episodeID;`,
	// Upsert recording
	"insertError": `
INSERT INTO error (
  recordingID,
  showID,
  episodeID,
  channelID,
  airDate,
  airingDuration,
  recordingDuration,
  recordingSize,
  recordingState,
  clean,
  comSkipState,
  comSkipError,
  errorCode,
  errorDetails,
  errorDescription
)
VALUES
%s;`,
	// Update systemInfo
	"updateSystemInfo": `
UPDATE systemInfo
SET
  serverName = '%s',
  privateIP = '%s';`,
	// Update space in systemInfo
	"updateSpace": `
UPDATE systemInfo
SET
  totalSize = %d,
  freeSize = %d;`,
	// Insert conflicts
	"insertConflicts": `
INSERT INTO scheduleConflicts (
  airingID,
  showID,
  airDate,
  endDate
)
VALUES
%s;`,
	// Delete exported
	"deleteExported": `
DELETE FROM exported
WHERE fullPath in ('%s');`,
	// Insert exported
	"insertExported": `
INSERT INTO exported (
  fullPath
)
VALUES
('%s')
ON CONFLICT DO NOTHING;`,
	// Select query record with specific action
	"selectQueueRecordByAction": `
SELECT
  count(*)
FROM
  queue
WHERE
  action = '%s';`,
	// Delete old airings
	"deleteExpiredAirings": `
DELETE FROM airing
WHERE
  airDate < %d;`,
	// Delete removed recordings
	"deleteRemovedRecordings": `
DROP TABLE IF EXISTS tempRecordingID;
CREATE TABLE tempRecordingID (
  recordingID INT NOT NULL PRIMARY KEY
);
INSERT INTO tempRecordingID (
  recordingID
)
VALUES
(%s);
DELETE FROM recording
WHERE
  recordingID IN (
    SELECT
      r.recordingID
    FROM
      recording r
      LEFT JOIN tempRecordingID t ON r.recordingID = t.recordingID
    WHERE
      t.recordingID IS NULL
  );
DROP TABLE IF EXISTS tempRecordingID;`,
	// Delete airing by airingID
	"deleteAiringByID": `
DELETE airing
WHERE airingID IN (%s);`,
}
