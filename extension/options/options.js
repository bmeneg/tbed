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
	if (idx >= 0)
    	return path.substr(idx+1);

	// Windows-based path
  	idx = path.lastIndexOf('\\');
  	if (idx >= 0)
    	return path.substr(idx+1);

	return path;
}

/* updateUI updates the field that shows the previous assigned editor path. */
async function updateSavedUI() {
	let storage;
	try {
		storage = await browser.storage.local.get();
	} catch(e) {
		console.error(e);
		return;
	}

	ui.savedEditor.innerText = storage.tbedEditor || "not selected";
	if (storage.tbedFromPath) {
		ui.args.value = storage.tbedEditor.split(" ", 1)[1] || "";
		ui.selectByPath.checked = true;
	} else {
		ui.shell.value = storage.tbedEditor || "";
		ui.selectByShell.checked = true;
	}
	toggleSelection();
}

/* optsSave concatenates both editor path and arguments field in a single
 * field into TB's local storage to be read later. */
function optsSave(event) {
	event.preventDefault();

	let cmd, fromPath;
	if (ui.selectByPath.checked) {
		path = extractFilename(ui.path.value);
		cmd = path.concat(" ", ui.args.value);
		fromPath = true;
	} else {
		cmd = ui.shell.value;
		fromPath = false;
	}

	browser.storage.local.set({
		tbedEditor: cmd,
		tbedFromPath: fromPath
	});
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
