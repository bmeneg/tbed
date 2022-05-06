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

// Development debug flag.
const DEBUG = true;

// Message length limits.
const EXT_MAX_MSGLEN = 4294967296;
const APP_MAX_MSGLEN = 1048576;

// Custom header delimiter.
const TBED_HEADER = "--tbed-hdr";

// Tab ID from where the message was first picked up.
var g_tabID = 0;

// Handle paged responses.
var g_pagedResponses = [];
var g_pages = 0;

/* Logging helper. */
function dbg(text)
{
	if (DEBUG) console.log(`tbed: dbg: ${text}`);
}

/* editor gets the external editor command set by the user in the options
 * page. */
async function editor() {
	let storage;
	try {
		storage = await browser.storage.local.get();
	} catch(e) {
		console.error(e)
	}

	return storage.tbedEditor;
}

/* hndlResponse is the event handler for responses coming from the native
 * application. */
function hndlResponse(resp)
{
	dbg(`resp: message: ${resp}`);
	
	// Handle possible paged/continued responses.
	if (resp.match(TBED_HEADER)) {
		if (resp.match("Pages: ")) {
			g_pages = resp.replace(/.*Pages: (\d+).*/g, "$1");
			dbg(`resp: message with ${g_pages} pages`);
			return;
		}
	}

	if (g_pages > 0) {
		dbg(`resp: continued response: ${resp}`);
		g_pagedResponses.push(resp);
		g_pages--;
		return;
	}

	// Return the resp as-is if there were not CONT messages.
	let finalResp = resp;
	if (g_pagedResponses.length > 0) {
		finalResp = g_pagedResponses.join('').concat(resp);
		g_pagedResponses = [];
		return resp;
	}

	browser.compose.setComposeDetails(g_tabID, {plainTextBody: finalResp});
}

/* sendMessage is a simple wrapper around WebExtension's API postMessage()
 * to ease debugging and preventing weird user behavior.
 * Note: we need to make it async because accessing the storage on editor()
 * is an async operation and we can't move forward without the result. */
async function sendMessage(appPort, msg)
{
	// It should _never_ happen. 4GB _should_ not be a real message
	if (msg.length > EXT_MAX_MSGLEN) {
		console.error("no, you won't send a freaking 4G+ payload");
		return;
	}

	// First send the editor command to be ran by the native app.
	const cmd = await editor();
	cmdMsg = `${TBED_HEADER}\nCommand: ${cmd}`;
	dbg(`send: message len: ${cmdMsg.length}`);
	dbg(`send: command message: ${cmdMsg}`);
	appPort.postMessage(cmdMsg);

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
		console.error(`connection failed: ${appPort.error.message}`);
		return;
	}
	dbg("connected with success");

	appPort.onMessage.addListener(hndlResponse);
	return appPort;
}

/* hndlEvent is the common code for extracting the compose text to be sent
 * to the native application. */
async function hndlUIEvent(tabObj) {
	dbg(`tab id: ${tabObj.id} tab title: ${tabObj.title}`);
	g_tabID = tabObj.id
	let appPort = initNativeConnection();
	
	let cDetails = await browser.compose.getComposeDetails(g_tabID);
	if (cDetails.isPlainText == false) {
		console.error("non-plaintext isn't supported yet");
		return;
	}

	let body = cDetails.plainTextBody;
	await sendMessage(appPort, body);
	return;
}

/* btnClicked is just a wrapper around hndlEvent for easing debug when
 * the button in the compose window is clicked. */
function btnClicked(tabObj)
{
	dbg("event: button clicked");
	hndlUIEvent(tabObj);
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
		console.error("impossible to know which tab is the correct");
		return;
	}
	hndlUIEvent(fWindow.tabs[0]);
}

/* setupListeners set the event handlers for the extension. */
function setupListeners() {
	browser.composeAction.onClicked.addListener(btnClicked);
	browser.commands.onCommand.addListener(cmdCalled);
}

setupListeners();