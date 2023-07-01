var postActions = function() {
	var $container = He.get('moving');

	var tr = function(term, str){
		return term.replace("%s", str);
	};
	var MultiMove = function(el, id, singleUser, loc) {
		var lbl = el.options[el.selectedIndex].textContent;
		var loc = JSON.parse(loc);
		var collAlias = el.options[el.selectedIndex].value;
		var $lbl = He.$('label[for=move-'+id+']')[0];
		//$lbl.textContent = loc['moving to']+" "+lbl+"...";
		$lbl.textContent = tr(loc['moving to %s...'],lbl);

		var params;
		if (collAlias == '|anonymous|') {
			params = [id];
		} else {
			params = [{
				id: id
			}];
		}
		var callback = function(code, resp) {
			if (code == 200) {
				for (var i=0; i<resp.data.length; i++) {
					if (resp.data[i].code == 200) {
						//$lbl.innerHTML = loc["Moved to"]+" <strong>"+lbl+"</strong>";
						$lbl.innerHTML = tr(loc["Moved to %s"], " <strong>"+lbl+"</strong>");
						var pre = "/"+collAlias;
						if (typeof singleUser !== 'undefined' && singleUser) {
							pre = "";
						}
						var newPostURL = pre+"/"+resp.data[i].post.slug;
						try {
							// Posts page
							He.$('#post-'+resp.data[i].post.id+' > h3 > a')[0].href = newPostURL;
						} catch (e) {
							// Blog index
							var $article = He.get('post-'+resp.data[i].post.id);
							$article.className = 'norm moved';
							if (collAlias == '|anonymous|') {
								var draftPre = "";
								if (typeof singleUser !== 'undefined' && singleUser) {
									draftPre = "d/";
								}
								$article.innerHTML = '<p><a href="/'+draftPre+resp.data[i].post.id+'">'+loc["Unpublished post"]+'</a>.</p>';
							} else {
								//$article.innerHTML = '<p>'+loc["Moved to"]+' <a style="font-weight:bold" href="'+newPostURL+'">'+lbl+'</a>.</p>';
								$article.innerHTML = '<p>'+ tr(loc["Moved to %s"], '<a style="font-weight:bold" href="'+newPostURL+'">'+lbl+'</a>')+'.</p>';
							}
						}
					} else {
						$lbl.innerHTML = loc['unable to move']+": "+resp.data[i].error_msg;
					}
				}
			}
		};
		if (collAlias == '|anonymous|') {
			He.postJSON("/api/posts/disperse", params, callback);
		} else {
			He.postJSON("/api/collections/"+collAlias+"/collect", params, callback);
		}
	};
	var Move = function(el, id, collAlias, singleUser, loc) {
		var lbl = el.textContent;
		var loc = JSON.parse(loc)

		/*
		try {
			//var m = lbl.match(/move to (.*)/);
			var m = lbl.match(RegExp(loc['move to'] + "(.*)"));
			lbl = m[1];
		} catch (e) {
			if (collAlias == '|anonymous|') {
				lbl = "draft";
			}
		}
		*/
		if (collAlias == '|anonymous|'){
			lbl = loc["draft"];
		}else{
			lbl = collAlias
		}

		el.textContent = tr(loc['moving to %s...'],lbl);
		if (collAlias == '|anonymous|') {
			params = [id];
		} else {
			params = [{
				id: id
			}];
		}
		var callback = function(code, resp) {
			if (code == 200) {
				for (var i=0; i<resp.data.length; i++) {
					if (resp.data[i].code == 200) {
						el.innerHTML = tr(loc["Moved to %s"], " <strong>"+lbl+"</strong>");
						el.onclick = null;
						var pre = "/"+collAlias;
						if (typeof singleUser !== 'undefined' && singleUser) {
							pre = "";
						}
						var newPostURL = pre+"/"+resp.data[i].post.slug;
						el.href = newPostURL;
						el.title = tr(loc["View on %s"], lbl)
						try {
							// Posts page
							He.$('#post-'+resp.data[i].post.id+' > h3 > a')[0].href = newPostURL;
						} catch (e) {
							// Blog index 
							var $article = He.get('post-'+resp.data[i].post.id);
							$article.className = 'norm moved';
							if (collAlias == '|anonymous|') {
								var draftPre = "";
								if (typeof singleUser !== 'undefined' && singleUser) {
									draftPre = "d/";
								}
								$article.innerHTML = '<p><a href="/'+draftPre+resp.data[i].post.id+'">'+loc["Unpublished post"]+'</a>.</p>';
							} else {
								$article.innerHTML = '<p>'+ tr(loc["Moved to %s"], '<a style="font-weight:bold" href="'+newPostURL+'">'+lbl+'</a>')+'.</p>';
							}
						}
					} else {
						el.innerHTML = loc['unable to move']+": "+resp.data[i].error_msg;
					}
				}
			}
		}
		if (collAlias == '|anonymous|') {
			He.postJSON("/api/posts/disperse", params, callback);
		} else {
			He.postJSON("/api/collections/"+collAlias+"/collect", params, callback);
		}

	};

	return {
		move: Move,
		multiMove: MultiMove,
	};
}();
