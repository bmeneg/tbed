/* Gather all elements in the options HTML by their ID. */
var ui = {};
for (const element of document.querySelectorAll("[id]")) {
	ui[element.id] = element;
}

/* extractFilename handle the fakepath passed by the API to the extension when
 * reading the file input element's value. Reference: 
 * https://html.spec.whatwg.org/multipage/input.html#fakepath-srsly */
function extractFilename(path) {
	// modern browser
	if (path.substr(0, 12) == "C:\\fakepath\\")
    	return path.substr(12);

	let idx;
	// Unix-based path
	idx = path.lastIndexOf('/');
  	if (x >= 0)
    	return path.substr(idx+1);

	// Windows-based path
  	idx = path.lastIndexOf('\\');
  	if (idx >= 0)
    	return path.substr(idx+1);

	return path;
}

function onError(e) {
	console.error(e);
}

/* updateUI updates the field that shows the previous assigned editor path. */
async function updateSavedUI() {
	const storage = await browser.storage.local.get();

	ui.savedEditor.innerText = storage.tbed_editor_cmd || "not selected";

	if (ui.args.enabled) {
		ui.args.value = storage.tbed_editor_cmd.split(" ", 1)[1] || "";
	} else {
		ui.shell.value = storage.tbed_editor_cmd || "";
	}
}

/* optsSave concatenates both editor path and arguments field in a single
 * field into TB's local storage to be read later. */
function optsSave(event) {
	event.preventDefault();

	let cmd;
	if (ui.path.enabled) {
		path = extractFilename(ui.path.value);
		cmd = path.concat(" ", ui.args.value);
	} else {
		cmd = ui.shell.value;
	}

	browser.storage.local.set({tbed_editor_cmd: cmd});
	updateSavedUI();
}

/* toggleSelection handles the radio button logic. */
function toggleSelection() {
	if (ui.selectByPath.checked) {
		ui.shell.disabled = true;
		ui.path.disabled = false;
		ui.args.disabled = false;
	} else {
		ui.path.disabled = true;
		ui.args.disabled = true;
		ui.shell.disabled = false;
	}
}

ui.editorOpts.addEventListener("submit", optsSave);
ui.selectByPath.addEventListener("change", toggleSelection);
ui.selectByShell.addEventListener("change", toggleSelection);
updateSavedUI();