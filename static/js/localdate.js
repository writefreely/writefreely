function toLocalDate(dateEl, displayEl) {
	var d = new Date(dateEl.getAttribute("datetime"));
	displayEl.textContent = d.toLocaleDateString(navigator.language || "en-US", { year: 'numeric', month: 'long', day: 'numeric' });
}

// Adjust dates on individual post pages, and on posts in a list *with* an explicit title
var $dates = document.querySelectorAll("article > time");
for (var i=0; i < $dates.length; i++) {
	toLocalDate($dates[i], $dates[i]);
}

// Adjust dates on posts in a list without an explicit title, where they act as the header
$dates = document.querySelectorAll("h2.post-title > time");
for (i=0; i < $dates.length; i++) {
	toLocalDate($dates[i], $dates[i].querySelector('a'));
}