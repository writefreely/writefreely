/**
 * Functionality for managing local Write.as posts.
 *
 * Dependencies:
 *   h.js
 */
function toggleTheme() {
	var btns;
	try {
		btns = Array.prototype.slice.call(document.getElementById('belt').querySelectorAll('.tool img'));
	} catch (e) {}
	if (document.body.className == 'light') {
		document.body.className = 'dark';
		try {
			for (var i=0; i<btns.length; i++) {
				btns[i].src = btns[i].src.replace('_dark@2x.png', '@2x.png');
			}
		} catch (e) {}
	} else if (document.body.className == 'dark') {
		document.body.className = 'light';
		try {
			for (var i=0; i<btns.length; i++) {
				btns[i].src = btns[i].src.replace('@2x.png', '_dark@2x.png');
			}
		} catch (e) {}
	} else {
		// Don't alter the theme
		return;
	}
	H.set('padTheme', document.body.className);
}
if (H.get('padTheme', 'light') != 'light') {
	toggleTheme();
}

var deleting = false;
function delPost(e, id, owned) {
	e.preventDefault();
	if (deleting) {
		return;
	}

	// TODO: UNDO!
	if (window.confirm('Are you sure you want to delete this post?')) {
		var token;
		for (var i=0; i<posts.length; i++) {
			if (posts[i].id == id) {
				token = posts[i].token;
				break;
			}
		}
		if (owned || token) {
			// AJAX
			deletePost(id, token, function() {
				// Remove post from list
				var $postEl = document.getElementById('post-' + id);
				$postEl.parentNode.removeChild($postEl);

				if (posts.length == 0) {
					displayNoPosts();
					return;
				}

				// Fill in full page of posts
				var $postsChildren = $posts.el.getElementsByClassName('post');
				if ($postsChildren.length < postsPerPage && $postsChildren.length < posts.length) {
					var lastVisiblePostID = $postsChildren[$postsChildren.length-1].id;
					lastVisiblePostID = lastVisiblePostID.substr(lastVisiblePostID.indexOf('-')+1);

					for (var i=0; i<posts.length-1; i++) {
						if (posts[i].id == lastVisiblePostID) {
							var $moreBtn = document.getElementById('more-posts');
							if ($moreBtn) {
								// Should always land here (?)
								$posts.el.insertBefore(createPostEl(posts[i-1]), $moreBtn);
							} else {
								$posts.el.appendChild(createPostEl(posts[i-1]));
							}
						}
					}
				}
			});
		} else {
			alert('Something went seriously wrong. Try refreshing.');
		}
	}
}
var getFormattedDate = function(d) {
	var mos = [
		"January", "February", "March",
		"April", "May", "June", "July",
		"August", "September", "October",
		"November", "December"
	];

	var day = d.getDate();
	var mo = d.getMonth();
	var yr = d.getFullYear();
	return mos[mo] + ' ' + day + ', ' + yr;
};
var posts = JSON.parse(H.get('posts', '[]'));

var initialListPop = function() {
	pages = Math.ceil(posts.length / postsPerPage);

	loadPage(page, true);
};

var $posts = H.getEl("posts");
if ($posts.el == null) {
	$posts = H.getEl("unsynced-posts");
}
$posts.el.innerHTML = '<p class="status">Reading...</p>';
var createMorePostsEl = function() {
	var $more = document.createElement('div');
	var nextPage = page+1;
	$more.id = 'more-posts';
	$more.innerHTML = '<p><a href="#' + nextPage + '">More...</a></p>';

	return $more;
};

var localPosts = function() {
	var $delPost, lastDelPost, lastInfoHTML;
	var $info = He.get('unsynced-posts-info');

	var findPostIdx = function(id) {
		for (var i=0; i<posts.length; i++) {
			if (posts[i].id == id) {
				return i;
			}
		}
		return -1;
	};

	var DismissError = function(e, el) {
		e.preventDefault();
		var $errorMsg = el.parentNode.previousElementSibling;
		$errorMsg.parentNode.removeChild($errorMsg);
		var $errorMsgNav = el.parentNode;
		$errorMsgNav.parentNode.removeChild($errorMsgNav);
	};
	var DeletePostLocal = function(e, el, id) {
		e.preventDefault();
		if (!window.confirm('Are you sure you want to delete this post?')) {
			return;
		}
		var i = findPostIdx(id);
		if (i > -1) {
			lastDelPost = posts.splice(i, 1)[0];
			$delPost = H.getEl('post-'+id);
			$delPost.setClass('del-undo');
			var $unsyncPosts = document.getElementById('unsynced-posts');
			var visible = $unsyncPosts.children.length;
			for (var i=0; i < $unsyncPosts.children.length; i++) { // NOTE: *.children support in IE9+
				if ($unsyncPosts.children[i].className.indexOf('del-undo') !== -1) {
					visible--;
				}
			}
			if (visible == 0) {
				H.getEl('unsynced-posts-header').hide();
				// TODO: fix undo functionality and don't do the following:
				H.getEl('unsynced-posts-info').hide();
			}
			H.set('posts', JSON.stringify(posts));
			// TODO: fix undo functionality and re-add
			//lastInfoHTML = $info.innerHTML;
			//$info.innerHTML = 'Unsynced entry deleted. <a href="#" onclick="localPosts.undoDelete()">Undo</a>.';
		}
	};
	var UndoDelete = function() {
		// TODO: fix this header reappearing
		H.getEl('unsynced-posts-header').show();
		$delPost.removeClass('del-undo');
		$info.innerHTML = lastInfoHTML;
	};

	return {
		dismissError: DismissError,
		deletePost: DeletePostLocal,
		undoDelete: UndoDelete,
	};
}();
var movePostHTML = function(postID) {
	let $tmpl = document.getElementById('move-tmpl');
	if ($tmpl === null) {
		return "";
	}
	return $tmpl.innerHTML.replace(/POST_ID/g, postID);
}
var createPostEl = function(post, owned) {
	var $post = document.createElement('div');
	var title = (post.title || post.id);
	title = title.replace(/</g, "&lt;");
	$post.id = 'post-' + post.id;
	$post.className = 'post';
	$post.innerHTML = '<h3><a href="/' + post.id + '">' + title + '</a></h3>';

	var posted = "";
	if (post.created) {
		posted = getFormattedDate(new Date(post.created))
	}
	var hasDraft = H.exists('draft' + post.id);
	$post.innerHTML += '<h4><date>' + posted + '</date> <a class="action" href="/pad/' + post.id + '">edit' + (hasDraft ? 'ed' : '') + '</a> <a class="delete action" href="/' + post.id + '" onclick="delPost(event, \'' + post.id + '\'' + (owned === true ? ', true' : '') + ')">delete</a> '+movePostHTML(post.id)+'</h4>';

	if (post.error) {
		$post.innerHTML += '<p class="error"><strong>Sync error:</strong> ' + post.error + ' <nav><a href="#" onclick="localPosts.dismissError(event, this)">dismiss</a> <a href="#" onclick="localPosts.deletePost(event, this, \''+post.id+'\')">remove post</a></nav></p>';
	}
	if (post.summary) {
		$post.innerHTML += '<p>' + post.summary.replace(/</g, "&lt;") + '</p>';
	} else if (post.body) {
		var preview;
		if (post.body.length > 140) {
			preview = post.body.substr(0, 140) + '...';
		} else {
			preview = post.body;
		}
		$post.innerHTML += '<p>' + preview.replace(/</g, "&lt;") + '</p>';
	}
	return $post;
};
var loadPage = function(p, loadAll) {
	if (loadAll) {
		$posts.el.innerHTML = '';
	}

	var startPost = posts.length - 1 - (loadAll ? 0 : ((p-1)*postsPerPage));
	var endPost = posts.length - 1 - (p*postsPerPage);
	for (var i=startPost; i>=0 && i>endPost; i--) {
		$posts.el.appendChild(createPostEl(posts[i]));
	}

	if (loadAll) { 
		if (p < pages) {
			$posts.el.appendChild(createMorePostsEl());
		}
	} else {
		var $moreEl = document.getElementById('more-posts');
		$moreEl.parentNode.removeChild($moreEl);
	}
	try {
		postsLoaded(posts.length);
	} catch (e) {}
};
var getPageNum = function(url) {
	var hash;
	if (url) {
		hash = url.substr(url.indexOf('#')+1);
	} else {
		hash = window.location.hash.substr(1);
	}

	var page = hash || 1;
	page = parseInt(page);
	if (isNaN(page)) {
		page = 1;
	}

	return page;
};

var postsPerPage = 10;
var pages = 0;
var page = getPageNum();

window.addEventListener('hashchange', function(e) {
	var newPage = getPageNum();
	var didPageIncrement = newPage == getPageNum(e.oldURL) + 1;

	loadPage(newPage, !didPageIncrement);
});

var deletePost = function(postID, token, callback) {
	deleting = true;

	var $delBtn = document.getElementById('post-' + postID).getElementsByClassName('delete action')[0];
	$delBtn.innerHTML = '...';

	var http = new XMLHttpRequest();
	var url = "/api/posts/" + postID + (typeof token !== 'undefined' ? "?token=" + encodeURIComponent(token) : '');
	http.open("DELETE", url, true);
	http.onreadystatechange = function() {
		if (http.readyState == 4) {
			deleting = false;
			if (http.status == 204 || http.status == 404) {
				for (var i=0; i<posts.length; i++) {
					if (posts[i].id == postID) {
						// TODO: use this return value, along will full content, for restoring post
						posts.splice(i, 1);
						break;
					}
				}
				H.set('posts', JSON.stringify(posts));

				callback();
			} else if (http.status == 409) {
				$delBtn.innerHTML = 'delete';
				alert("Post is synced to another account. Delete the post from that account instead.");
				// TODO: show "remove" button instead of "delete" now
				// Persist that state.
				// Have it remove the post locally only.
			} else {
				$delBtn.innerHTML = 'delete';
				alert("Failed to delete. Please try again.");
			}
		}
	}
	http.send();
};

var hasWritten = H.get('lastDoc', '') !== '';

var displayNoPosts = function() {
	if (auth) {
		$posts.el.innerHTML = '';
		return;
	}
	var cta = '<a href="/pad">Create a post</a> and it\'ll appear here.';
	if (hasWritten) {
		cta = '<a href="/pad">Finish your post</a> and it\'ll appear here.';
	}
	H.getEl("posts").el.innerHTML = '<p class="status">No posts created yet.</p><p class="status">' + cta + '</p>';
};

if (posts.length == 0) {
	displayNoPosts();
} else {
	initialListPop();
}

