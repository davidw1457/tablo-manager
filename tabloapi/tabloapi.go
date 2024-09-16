package tabloapi

type WebApiResp struct {
	Cpes []TabloDetails
}

type TabloDetails struct {
	Serverid   string
	Name       string
	Private_ip string
}

type Channel struct {
	Object_id int
	Channel   ChannelDetails
}
type ChannelDetails struct {
	Call_sign string
	Major     int
	Minor     int
	Network   string
}

type Show struct {
	Object_id int
	Schedule  ScheduleDetails
	Series    SeriesDetails
	Movie     MovieDetails
	Sport     SportDetails
	Keep      KeepDetails
}

type ScheduleDetails struct {
	Rule         string
	Channel_path string
}

type SeriesDetails struct {
	Title           string
	Description     string
	Orig_air_date   string
	Episode_runtime int
	Series_rating   string
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
	Count int
}

type MovieDetails struct {
	Title            string
	Plot             string
	Original_runtime int
	Release_year     int
	Film_rating      string
	Quality_rating   int
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
