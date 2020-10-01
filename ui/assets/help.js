function showAdditionalResourcesHeading() {
    let arh = document.getElementById("hAdditionalResources");
    arh.removeAttribute("class");
    arh.parentNode.replaceChild(arh, arh);
}

// setAdditionalResources replaces the old unordered list with ul
function setAdditionalResources(ar) {
    let arOld = document.getElementById("ulAdditionalResources");
    if (ar === null) {
        return;
    }
    arOld.parentNode.replaceChild(ar, arOld);
    ar.id = "ulAdditionalResources";
}

// renderExternalDocumentURLs renders the custom urls
function renderExternalDocumentURLs(urls) {
    let addedResources = document.getElementById("ulAdditionalResources");
    for (let i = 0; i < urls.length; i++) {
        let li = document.createElement("li");
        let a = document.createElement("a");
        a.innerHTML = urls[i];
        a.href = urls[i];
        li.appendChild(a);
        addedResources.appendChild(li);
    }
    setAdditionalResources(addedResources);
}

// getExternalDocumentURLs gets external document urls
function getExternalDocumentURLs(env_name) {
    let req = new XMLHttpRequest();

    req.open('GET', `${apiBaseURL}/v2/help/document/urls`, true);
    req.onload = function (e) {
        if (req.status !== 200) {
            console.log(`namespace pods request failed: ${req.status}: ${req.responseText}`);
            return;
        }

        let data = JSON.parse(req.response);
        if (data.hasOwnProperty('document_urls')) {
            if (Object.keys(data.document_urls).length > 0) {
                showAdditionalResourcesHeading();
                renderExternalDocumentURLs(data.document_urls);
            }
        }
    };
    req.onerror = function (e) {
        console.error(`error getting namespace pod endpoint for ${env_name}: ${req.statusText}`);
    };

    req.send();
}

document.addEventListener("DOMContentLoaded", function(){
    getExternalDocumentURLs();
});
