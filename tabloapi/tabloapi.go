package tabloapi

type WebApiResp struct {
	Cpes []TabloDetails
}

type TabloDetails struct {
	ServerID   string
	Name       string
	Private_IP string
}

type Channel struct {
	Object_ID int
	Channel   ChannelDetails
}
type ChannelDetails struct {
	Call_Sign string
	Major     int
	Minor     int
	Network   *string
}

type Show struct {
	Object_ID  int
	Path       string
	Schedule   ShowScheduleDetails
	Series     SeriesDetails
	Movie      MovieDetails
	Sport      SportDetails
	Keep       KeepDetails
	Guide_Path string
}

type ShowScheduleDetails struct {
	Rule         string
	Channel_Path string
}

type SeriesDetails struct {
	Title           string
	Description     *string
	Orig_Air_Date   *string
	Episode_Runtime int
	Series_Rating   *string
	Genres          []string
	Cast            []string
	Awards          []Award
}

type Award struct {
	Won      bool
	Name     string
	Category string
	Year     int
	Nominee  string
}

type KeepDetails struct {
	Rule  string
	Count *int
}

type MovieDetails struct {
	Title            string
	Plot             *string
	Original_Runtime int
	Release_Year     *int
	Film_Rating      *string
	Quality_Rating   *int
	Cast             []string
	Directors        []string
	Awards           []Award
	Genres           []string
}

type SportDetails struct {
	Title       string
	Description string
	Genres      []string
}

type Airing struct {
	Object_ID      int
	Series_Path    string
	Movie_Path     string
	Sport_Path     string
	Episode        EpisodeDetails
	Airing_Details AiringDetails
	Schedule       AiringScheduleDetails
	Event          EventDetails
}

type EpisodeDetails struct {
	Title         string
	Description   string
	Number        int
	Season_Number int
	Orig_Air_Date *string
}

type AiringDetails struct {
	Datetime string
	Duration int
	Channel  Channel
}

type AiringScheduleDetails struct {
	State string
}

type EventDetails struct {
	Title        string
	Description  string
	Season       string
	Season_Type  string
	Home_Team_ID *int
	Teams        []Team
}

type Team struct {
	Name    string
	Team_ID int
}

type Recording struct {
	Object_ID      int
	Series_Path    string
	Movie_Path     string
	Sport_Path     string
	Episode        EpisodeDetails
	Airing_Details AiringDetails
	Schedule       AiringScheduleDetails
	Event          EventDetails
	Video_Details  VideoDetails
}

type VideoDetails struct {
	State    string
	Clean    bool
	Size     int
	Duration int
	ComSkip  ComSkipDetails
	Error    ErrorDetails
}

type ComSkipDetails struct {
	State string
	Error *string
}

type ErrorDetails struct {
	Code        *string
	Details     *string
	Description *string
}

type Drive struct {
	Size int64
	Free int64
}
