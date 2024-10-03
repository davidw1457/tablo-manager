# tablo manager

A Go app for connecting to and managing a (pre gen-4) Tablo DVR

This is a re-write in Go of the app I was working on in Dart. The intention is to use this application as a backend to create and update a local database of the Tablo recordings and guide and to export out recordings. The app is intended to be portable so that I can leave it on 24/7 on a low power non-Windows device (either in a container on the NAS or on a Raspberry Pi).

I intend to create a Flutter appliction to operate as a front end for manually managing schedules and exports.

## DONE
Application will find all Tablos on the network and create a Sqlite database of all guide data and all recordings. By default, the database is stored in the users home directory in a folder called .tablomanager. You can specify a different destination as a commandline argument when launching (e.g. "tablomanager C:\MyTabloData" will create the database in C:\MyTabloData). The database is named by the internal Tablo serverID and ends with .cache (e.g. SID_01234567890A.cache). You can view the database contents with any Sqlite database manger. [DB Browser for SQLite](https://sqlitebrowser.org/) (DB4S) has worked well for me.

With the eventual front end you will be able to specify a default export directory, manually queue up exports to any chosen directory, delete recordings, browse shows not currently recorded and schedule them to record, and prioritize shows for automatic conflict resolution. Currently some of this can be done with DB4S.

If you set a valid default export path (in systemInfo.defaultExportPath), the program will scan that directory for any files and automatically unschedule any recordings that exist in the exports.

If you set priority in the showPriority table, the program will automatically resolve any conflicts, keeping the recordings with the lowest priority value. The table has two fields, showID (which can be found in the show table) and priority (an integer value). Movies automatically receive priority level 0 (highest priority). Any value below 0 is invalid and will result in an error that prevents automatic conflict resolution. Any shows with conflicts that do not have a priority set are treated as if they have a priority of -1, preventing automatic conflict resolution. This is to prevent accidentally unscheduling shows that should have been higher priority.

## TODO
* Create function to export recordings
* Auto-queue exports for all recordings
* Auto-delete failed recordings
* Cache thumbnail images from Tablo to use in Flutter frontend
* Increase error handling when Tablo fails to respond
* Auto-reboot Tablo once/day when it is not recording (using Kasa smart powerstrip)
* Fall back to using UDP broadcast to find Tablo in case Nuvyyo takes down public site

## Thanks
* Thanks to jessedp for documenting the [Tablo API](https://github.com/jessedp/tablo-api-docs/blob/main/source/index.html.md)!