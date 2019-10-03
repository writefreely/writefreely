function toLocalDate(el) {
	var d = new Date(el.getAttribute("datetime"));
	el.textContent = d.toLocaleDateString(navigator.language || "en-US", { year: 'numeric', month: 'long', day: 'numeric' });
}

var $dates = document.querySelectorAll("time");
for (var i=0; i < $dates.length; i++) {
	toLocalDate($dates[i]);
}
