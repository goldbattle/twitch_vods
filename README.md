# twitch_vods

Some prototyping code for archiving twitch vods. Originally based on experiements from
[TimIsOverpowered/twitch-recorder-go](https://github.com/TimIsOverpowered/twitch-recorder-go) method of recording.
Converged on using streamlink to record the live streams and helix api to download vods and chat from already recorded videos.
Additionally, live chat recording was added which leverages irc to record.

- twitch_download_chat - Download vod chats and convert into the correct [TwitchDownloader](https://github.com/lay295/TwitchDownloader) format
- twitch_download_vod - Will poll for new vods to download, and download them after the specified time
- twitch_live_stream - Records live streams with streamlink and irc to record live chat into the correct format and live title & game changes

I don't support this code, just making public for those interested in doing it themselves.


