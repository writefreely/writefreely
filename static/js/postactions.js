var postActions = function() {
	var $container = He.get('moving');
	var MultiMove = function(el, id, singleUser) {
		var lbl = el.options[el.selectedIndex].textContent;
		var collAlias = el.options[el.selectedIndex].value;
		var $lbl = He.$('label[for=move-'+id+']')[0];
		$lbl.textContent = "moving to "+lbl+"...";
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
						$lbl.innerHTML = "moved to <strong>"+lbl+"</strong>";
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
								$article.innerHTML = '<p><a href="/'+draftPre+resp.data[i].post.id+'">Unpublished post</a>.</p>';
							} else {
								$article.innerHTML = '<p>Moved to <a style="font-weight:bold" href="'+newPostURL+'">'+lbl+'</a>.</p>';
							}
						}
					} else {
						$lbl.innerHTML = "unable to move: "+resp.data[i].error_msg;
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
	var Move = function(el, id, collAlias, singleUser) {
		var lbl = el.textContent;
		try {
			var m = lbl.match(/move to (.*)/);
			lbl = m[1];
		} catch (e) {
			if (collAlias == '|anonymous|') {
				lbl = "draft";
			}
		}

		el.textContent = "moving to "+lbl+"...";
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
						el.innerHTML = "moved to <strong>"+lbl+"</strong>";
						el.onclick = null;
						var pre = "/"+collAlias;
						if (typeof singleUser !== 'undefined' && singleUser) {
							pre = "";
						}
						var newPostURL = pre+"/"+resp.data[i].post.slug;
						el.href = newPostURL;
						el.title = "View on "+lbl;
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
								$article.innerHTML = '<p><a href="/'+draftPre+resp.data[i].post.id+'">Unpublished post</a>.</p>';
							} else {
								$article.innerHTML = '<p>Moved to <a style="font-weight:bold" href="'+newPostURL+'">'+lbl+'</a>.</p>';
							}
						}
					} else {
						el.innerHTML = "unable to move: "+resp.data[i].error_msg;
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
