# twitch_vods

Some prototyping code for archiving twitch vods. Originally based on experiements from
[TimIsOverpowered/twitch-recorder-go](https://github.com/TimIsOverpowered/twitch-recorder-go) method of recording.
Converged on using streamlink to record the live streams and helix api to download vods and chat from already recorded videos.
Additionally, live chat recording was added which leverages irc to record.

- twitch_download_chat - Download vod chats and convert into the correct [TwitchDownloader](https://github.com/lay295/TwitchDownloader) format
- twitch_download_vod - Will poll for new vods to download, and download them after the specified time
- twitch_live_stream - Records live streams with streamlink and irc to record live chat into the correct format and live title & game changes

I don't support this code, just making public for those interested in doing it themselves.


## Live Video + Chat Recording

This logic relies on having streamlink installed on your system along with ffmpeg.
After detecting that a stream specified in `channels_live` is live, it will first try to find a VOD id which matches to the current stream id.
This should always happen unless the streamer has disabled their VODs.
As a fallback then the stream ID should be used, which is a distinct number.

Streamlink recording is then started with any user specified commands.
If you have Twitch Prime, then an Oauth token can be [specified](https://streamlink.github.io/cli/plugins/twitch.html#authentication) which should remove all ads in the video stream.
This is pretty much necessary as there is no way to remove the embedded ads via streamlink, just to remove them from the save file which breaks continuity.

```json
"streamlink_options": [
    "--twitch-disable-hosting",
    "--twitch-disable-reruns",
    "--twitch-api-header=Authorization=OAuth XXXXXXXXXXXXXXXXXXXXXXX",
    "--twitch-low-latency"
]
```

To get the Oauth token, one can run this on the Twitch website to get its value:
```javascript
document.cookie.split("; ").find(item=>item.startsWith("auth-token="))?.split("=")[1]
```

To record the chat, the system connects to the IRC after streamlink has created a file and will start parsing the IRC messages into the [TwitchDownloader](https://github.com/lay295/TwitchDownloader) format.
There is additionally support for "live chat" recording via the `channels_live_chat` config, which still requires streamlink, but will record a very small "worst quality" stream along side the chat.
This is to reduce the file storage needed if just chat archiving alongside audio is desired.

Additionally, a third thread will constantly check for title and game changes, which will be recorded into the information json file.
The last step after a stream is finished (detected when the streamlink process exits) is to transcode the streamlink video recording so that the mp4 recorded is valid.
This is done by just running ffmpeg over the whole video inplace to do any corrections.
