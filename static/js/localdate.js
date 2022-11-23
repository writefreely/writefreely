//localdate.js
//Modified for correctly formating the dates in Basque language(while some errors are fixed on "unicode-org/cldr" 
// https://github.com/unicode-org/cldr/blob/main/common/main/eu.xml): 2022/08/16

function toLocalDate(dateEl, displayEl, longFormat) {
	var d = new Date(dateEl.getAttribute("datetime"));

	//displayEl.textContent = d.toLocaleDateString(navigator.language || "en-US", { year: 'numeric', month: 'long', day: 'numeric' });
	if(longFormat){
		var dateString = d.toLocaleDateString(navigator.language || "en-US", { year: 'numeric', month: 'long', day: 'numeric' , hour: 'numeric', minute: 'numeric'});
	}else{
		var dateString = d.toLocaleDateString(navigator.language || "en-US", { year: 'numeric', month: 'long', day: 'numeric' });
	}

	function euFormat(data){
		var hk = [
			"urtarrila", "otsaila", "martxoa",
			"apirila", "maiatza", "ekaina", "uztaila",
			"abuztua", "iraila", "urria",
			"azaroa", "abendua"
		];

		var e = data.getDate();
		var h = data.getMonth();
		var u = data.getFullYear();
		var or = data.getHours();
		var min = data.getMinutes();
		var a = (or >= 12) ? "PM" : "AM";

		var lot = ((((Number(u[2])%2 == 0 && u[3] =='1') || (Number(u[2])%2 == 1 && u[3]=='0')) || u[3] == '5')?'e':'')+'ko'

		if(longFormat){
			return u + lot + ' ' + hk[h] + 'k ' + e + ', ' + or + ':' + min + ' ' + a;
		}else{
			return u + lot + ' ' + hk[h] + 'k ' + e;
		}

		
	}

	//if lang eu or eu-ES ...
	displayEl.textContent = navigator.language.indexOf('eu') != -1 ? euFormat(d) : dateString

}

// Adjust dates on individual post pages, and on posts in a list *with* an explicit title
var $dates = document.querySelectorAll("article > time");
for (var i=0; i < $dates.length; i++) {
	toLocalDate($dates[i], $dates[i]);
}

// Adjust dates on posts in a list without an explicit title, where they act as the header
$dates = document.querySelectorAll("h2.post-title > time");
for (i=0; i < $dates.length; i++) {
	toLocalDate($dates[i], $dates[i].querySelector('a'), false);
}

// Adjust dates on drafts 2022/08/16
$dates = document.querySelectorAll("h4 > date");
for (var i=0; i < $dates.length; i++) {
	$dates[i].setAttribute("datetime", $dates[i].getAttribute("datetime").split(" +")[0])
	toLocalDate($dates[i], $dates[i], false);
}

// Adjust date on privacy page 2022/08/17
$dates = document.querySelectorAll("p > time");
for (var i=0; i < $dates.length; i++) {
	toLocalDate($dates[i], $dates[i], false);
}

// Adjust date to long format on admin/user-s pages 2022/11/21
if(location.pathname.startsWith("/admin/user")){
	$dates = document.querySelectorAll("td > time");
	for (var i=0; i < $dates.length; i++) {
		toLocalDate($dates[i], $dates[i], true);
	}
}
