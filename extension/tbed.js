/*
 * ThunderBird Editor is an extension that allows the user to compose the
 * email in an external editor of his choice. The path to the editor should be
 * added in the extension's option page avoiding messing with platform
 * particularities.
 *
 * Author: Bruno Meneguele <bmeneguele@gmail.com>
 */

/*
 * This extension uses the MailExtension architecture that was introduced with
 * ThunderBird 78. This standard interacts directly with the WebExtension API
 * from Mozilla Firefox with specific mail-based features, like handling with
 * the compose window/text that we're going to need.
 *
 * The WebExtension API has the feature to talk with a native application
 * installed in the host's machine by sending and receiving specific messages
 * in a connection (connectionless is also supported, but we won't used it
 * here) manner. The native application is defined through the NativeMessaging
 * standard that defines a manifest file stored in a specific location
 * (platform dependent), which specify the path/command of the native
 * application.
 *
 * This transaction has some limitations and also follows a simple protocol:
 * - Limitation:
 *   - messages from the extension to the native application has maximum size
 *   of 4GB.
 *	 - messages from the native application to the extension has maximum size
 *	 of 1MB.
 *
 * - Protocol: 
 *   - header: 4 bytes indicating the size of message's payload in an uint32
 *   format.
 *   - payload: must be json streamed.
 */
 
// Message length limits.
const EXT_MAX_MSGLEN = 4294967296;
const APP_MAX_MSGLEN = 1048576;

var g_debug = true;
var g_tabID = 0;

// Handle paged responses.
var g_paged_responses = [];
var g_pages = 0;

/* Logging helpers. */
function err(text)
{
	console.log(`tbed: err: ${text}`);
}

function info(text)
{
	console.log(`tbed: info: ${text}`);
}

function dbg(text)
{
	if (g_debug) console.log(`tbed: dbg: ${text}`);
}

/* hndlResponse is the event handler for responses coming from the native
 * application. */
function hndlResponse(resp)
{
	dbg(`resp: message: ${resp}`);
	
	// Handle possible paged/continued responses.
	customHeader = "--tbed-hdr\n"
	if (resp.startsWith(customHeader)) {
		if (resp.substring(customHeader.length).startsWith("Pages:")) {
			g_pages = resp.substring(4 + "Pages: ".length)
			dbg(`resp: message with ${pages} pages`)
			return
		}
	}

	if (g_pages > 0) {
		dbg(`resp: continued response: ${resp}`);
		g_paged_responses.push(resp);
		g_pages--
		return;
	}

	// Return the resp as-is if there were not CONT messages.
	let finalResp = resp
	if (g_paged_responses.length > 0) {
		finalResp = g_paged_responses.join('').concat(resp);
		g_paged_responses = [];
		return resp;
	}

	browser.compose.setComposeDetails(g_tabID, {plainTextBody: finalResp})
}

/* sendMessage is a simple wrapper around WebExtension's API postMessage()
 * to ease debugging and preventing weird user behavior. */
function sendMessage(appPort, msg)
{
	// It should _never_ happen. 4GB _should_ not be a real message
	if (msg.length > EXT_MAX_MSGLEN) {
		err("no, you won't send a freaking payload with more than 4GB");
		return;
	}

	dbg(`send: message len: ${msg.length}`);
	dbg(`send: message: ${msg}`);
	appPort.postMessage(msg)
}

/* initNativeConnection establishes the connection with the native application
 * on host's system. The native app name is defined in app's manifest. */
function initNativeConnection()
{
	appPort = browser.runtime.connectNative("tbed");
	if (appPort.error) {
		err(`failed to connect to application: ${appPort.error.message}`);
		return;
	}
	dbg("connection: success");

	appPort.onMessage.addListener(hndlResponse);

	return appPort;
}

/* hndlEvent is the common code for extracting the compose text to be sent
 * to the native application. */
async function hndlEvent(tabObj) {
	dbg(`tab id: ${tabObj.id} tab title: ${tabObj.title}`);
	g_tabID = tabObj.id

	let appPort = initNativeConnection();
	
	let cDetails = await browser.compose.getComposeDetails(g_tabID);
	if (cDetails.isPlainText == false) {
		err("unfortunately non-plaintext isn't supported yet");
		return;
	}

	let body = cDetails.plainTextBody;
	sendMessage(appPort, body);
}

/* btnClicked is just a wrapper around hndlEvent for easing debug when
 * the button in the compose window is clicked. */
function btnClicked(tabObj)
{
	dbg("event: button clicked");
	hndlEvent(tabObj);
}

/* cmdCalled handle hotkey command events. */
async function cmdCalled(event)
{
	// Hotkey event name is defined in extension's manifest.
	if (event != "tbed") return;
	dbg("event: hotkey command called");

	// We need the tab ID that contains the compose text, however, on command
	// events the tab object is not passed in any way, so we need to traverse
	// all open windows to gather it ourselves.
	let windows = await browser.windows.getAll({
		populate: true,
		windowTypes: ["messageCompose"]
	});

	let fWindow;
	for (const window of windows) {
		if (window.focused) {
			fWindow = window;
			break;
		}
	}
	dbg(`event: window id: ${fWindow.id}`);

	// Make sure our messageCompose has a single tab.
	if (fWindow.tabs.length != 1) {
		err("impossible to know what tab is the correct");
		return;
	}
	hndlEvent(fWindow.tabs[0]);
}

/* setupListeners set the event handlers for the extension. */
function setupListeners() {
	browser.composeAction.onClicked.addListener(btnClicked);
	browser.commands.onCommand.addListener(cmdCalled);
}

setupListeners();