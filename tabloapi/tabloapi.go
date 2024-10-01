package tabloapi

type WebAPIResp struct {
	Cpes []TabloDetails `json:"cpes"`
}

type TabloDetails struct {
	ServerID  string `json:"serverid"`
	Name      string `json:"name"`
	PrivateIP string `json:"private_ip"`
}

type Channel struct {
	ObjectID int            `json:"object_id"`
	Channel  ChannelDetails `json:"channel"`
}
type ChannelDetails struct {
	CallSign string  `json:"call_sign"`
	Major    int     `json:"major"`
	Minor    int     `json:"minor"`
	Network  *string `json:"network"`
}

type Show struct {
	ObjectID  int                 `json:"object_id"`
	Path      string              `json:"path"`
	Schedule  ShowScheduleDetails `json:"schedule"`
	Series    SeriesDetails       `json:"series"`
	Movie     MovieDetails        `json:"movie"`
	Sport     SportDetails        `json:"sport"`
	Keep      KeepDetails         `json:"keep"`
	GuidePath string              `json:"guide_path"`
}

type ShowScheduleDetails struct {
	Rule        string `json:"rule"`
	ChannelPath string `json:"channel_path"`
}

type SeriesDetails struct {
	Title          string   `json:"title"`
	Description    *string  `json:"description"`
	OrigAirDate    *string  `json:"orig_air_date"`
	EpisodeRuntime int      `json:"episode_runtime"`
	SeriesRating   *string  `json:"series_rating"`
	Genres         []string `json:"genres"`
	Cast           []string `json:"cast"`
	Awards         []Award  `json:"awards"`
}

type Award struct {
	Won      bool   `json:"won"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Year     int    `json:"year"`
	Nominee  string `json:"nominee"`
}

type KeepDetails struct {
	Rule  string `json:"rule"`
	Count *int   `json:"count"`
}

type MovieDetails struct {
	Title           string   `json:"title"`
	Plot            *string  `json:"plot"`
	OriginalRuntime int      `json:"original_runtime"`
	ReleaseYear     *int     `json:"release_year"`
	FilmRating      *string  `json:"film_rating"`
	QualityRating   *int     `json:"quality_rating"`
	Cast            []string `json:"cast"`
	Directors       []string `json:"directors"`
	Awards          []Award  `json:"awards"`
	Genres          []string `json:"genres"`
}

type SportDetails struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Genres      []string `json:"genres"`
}

type Airing struct {
	ObjectID      int                   `json:"object_id"`
	SeriesPath    string                `json:"series_path"`
	MoviePath     string                `json:"movie_path"`
	SportPath     string                `json:"sport_path"`
	Episode       EpisodeDetails        `json:"episode"`
	AiringDetails AiringDetails         `json:"airing_details"`
	Schedule      AiringScheduleDetails `json:"schedule"`
	Event         EventDetails          `json:"event"`
	Error         RequestError          `json:"error"`
}

type EpisodeDetails struct {
	Title        string  `json:"title"`
	Description  string  `json:"description"`
	Number       int     `json:"number"`
	SeasonNumber int     `json:"season_number"`
	OrigAirDate  *string `json:"orig_air_date"`
}

type AiringDetails struct {
	Datetime string  `json:"datetime"`
	Duration int     `json:"duration"`
	Channel  Channel `json:"channel"`
}

type AiringScheduleDetails struct {
	State string `json:"state"`
}

type EventDetails struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Season      string `json:"season"`
	SeasonType  string `json:"season_type"`
	HomeTeamID  *int   `json:"home_team_id"`
	Teams       []Team `json:"teams"`
}

type Team struct {
	Name   string `json:"name"`
	TeamID int    `json:"team_id"`
}

type Recording struct {
	ObjectID      int                   `json:"object_id"`
	SeriesPath    string                `json:"series_path"`
	MoviePath     string                `json:"movie_path"`
	SportPath     string                `json:"sport_path"`
	Episode       EpisodeDetails        `json:"episode"`
	AiringDetails AiringDetails         `json:"airing_details"`
	Schedule      AiringScheduleDetails `json:"schedule"`
	Event         EventDetails          `json:"event"`
	VideoDetails  VideoDetails          `json:"video_details"`
}

type VideoDetails struct {
	State    string         `json:"state"`
	Clean    bool           `json:"clean"`
	Size     int            `json:"size"`
	Duration int            `json:"duration"`
	ComSkip  ComSkipDetails `json:"comskip"`
	Error    ErrorDetails   `json:"error"`
}

type ComSkipDetails struct {
	State string  `json:"state"`
	Error *string `json:"error"`
}

type ErrorDetails struct {
	Code        *string `json:"code"`
	Details     *string `json:"details"`
	Description *string `json:"description"`
}

type Drive struct {
	Size int64 `json:"size"`
	Free int64 `json:"free"`
}

type RequestError struct {
	Code        string `json:"code"`
	Details     int    `json:"details"`
	Description string `json:"description"`
}
