/**
 * H.js
 * 
 * Lightweight, extremely bare-bones library for manipulating the DOM and
 * saving some typing.
 */

var Element = function(domElement) {
	this.el = domElement;
};

/**
 * Creates a toggle button that adds / removes the given class name from the
 * given element.
 *
 * @param {Element} $el - The element to modify.
 * @param {string} onClass - The class to add to the given element.
 * @param {function} onFunc - Additional actions when toggling on.
 * @param {function} offFunc - Additional actions when toggling off.
 */
Element.prototype.createToggle = function($el, onClass, onFunc, offFunc) {
	this.on('click', function(e) {
		if ($el.el.className === '') {
			$el.el.className = onClass;
			onFunc(new Element(this), e);
		} else {
			$el.el.className = '';
			offFunc(new Element(this), e);
		}
		e.preventDefault();
	}, false);
};
Element.prototype.on = function(event, func) {
	events = event.split(' ');
	var el = this.el;
	if (el == null) {
		console.error("Error: element for event is null");
		return;
	}
	var addEvent = function(e) {
		if (el.addEventListener) {
			el.addEventListener(e, func, false);
		} else if (el.attachEvent) {
			el.attachEvent(e, func);
		}
	};
	if (events.length === 1) {
		addEvent(event);
	} else {
		for(var i=0; i<events.length; i++) {
			addEvent(events[i]);
		}
	}
};
Element.prototype.setClass = function(className) {
	if (this.el == null) {
		console.error("Error: element to set class on is null");
		return;
	}
	this.el.className = className;
};
Element.prototype.removeClass = function(className) {
	if (this.el == null) {
		console.error("Error: element to remove class on is null");
		return;
	}
	var regex = new RegExp(' ?' + className, 'g');
	this.el.className = this.el.className.replace(regex, '');
};
Element.prototype.text = function(text, className) {
	if (this.el == null) {
		console.error("Error: element for setting text is null");
		return;
	}
	if (this.el.textContent !== text) {
		this.el.textContent = text;
		if (typeof className !== 'undefined') {
			this.el.className = this.el.className + ' ' + className;
		}
	}
};
Element.prototype.insertAfter = function(newNode) {
	if (this.el == null) {
		console.error("Error: element for insertAfter is null");
		return;
	}
	this.el.parentNode.insertBefore(newNode, this.el.nextSibling);
};
Element.prototype.remove = function() {
	if (this.el == null) {
		console.error("Didn't remove element");
		return;
	}
	this.el.parentNode.removeChild(this.el);
};
Element.prototype.hide = function() {
	if (this.el == null) {
		console.error("Didn't hide element");
		return;
	}
	this.el.className += ' effect fade-out';
};
Element.prototype.show = function() {
	if (this.el == null) {
		console.error("Didn't show element");
		return;
	}
	this.el.className += ' effect';
};


var H = {
	getEl: function(elementId) {
		return new Element(document.getElementById(elementId));
	},
	save: function($el, key) {
		localStorage.setItem(key, $el.el.value);
	},
	load: function($el, key, onlyLoadPopulated) {
		var val = localStorage.getItem(key);
		if (onlyLoadPopulated && val == null) {
			// Do nothing
			return;
		}
		$el.el.value = val;
	},
	set: function(key, value) {
		localStorage.setItem(key, value);
	},
	get: function(key, defaultValue) {
		var val = localStorage.getItem(key);
		if (val == null) {
			val = defaultValue;
		}
		return val;
	},
	remove: function(key) {
		localStorage.removeItem(key);
	},
	exists: function(key) {
		return localStorage.getItem(key) !== null;
	},
	createPost: function(id, editToken, content, created) {
		var summaryLen = 200;
		var titleLen = 80;
		var getPostMeta = function(content) {
			var eol = content.indexOf("\n");
			if (content.indexOf("# ") === 0) {
				// Title is in the format:
				//
				//   # Some title
				var summary = content.substring(eol).trim();
				if (summary.length > summaryLen) {
					summary = summary.substring(0, summaryLen) + "...";
				}
				return {
					title: content.substring("# ".length, eol),
					summary: summary,
				};
			}

			var blankLine = content.indexOf("\n\n");
			if (blankLine !== -1 && blankLine <= eol && blankLine <= titleLen) {
				// Title is in the format:
				//
				//   Some title
				//
				//   The body starts after that blank line above it.
				var summary = content.substring(blankLine).trim();
				if (summary.length > summaryLen) {
					summary = summary.substring(0, summaryLen) + "...";
				}
				return {
					title: content.substring(0, blankLine),
					summary: summary,
				};
			}

			// TODO: move this to the beginning
			var title = content.trim();
			var summary = "";
			if (title.length > titleLen) {
				// Content can't fit in the title, so figure out the summary
				summary = title;
				title = "";
				if (summary.length > summaryLen) {
					summary = summary.substring(0, summaryLen) + "...";
				}
			} else if (eol > 0) {
				summary = title.substring(eol+1);
				title = title.substring(0, eol);
			}
			return {
				title: title,
				summary: summary
			};
		};
		
		var post = getPostMeta(content);
		post.id = id;
		post.token = editToken;
		post.created = created ? new Date(created) : new Date();
		post.client = "Pad";
		
		return post;
	},
	getTitleStrict: function(content) {
		var eol = content.indexOf("\n");
		var title = "";
		var newContent = content;
		if (content.indexOf("# ") === 0) {
			// Title is in the format:
			// # Some title
			if (eol !== -1) {
				// First line should start with # and end with \n
				newContent = content.substring(eol).leftTrim();
				title = content.substring("# ".length, eol);
			}
		}
		return {
			title: title,
			content: newContent
		};
	},
};

var He = {
	create: function(name) {
		return document.createElement(name);
	},
	get: function(id) {
		return document.getElementById(id);
	},
	$: function(selector) {
		var els = document.querySelectorAll(selector);
		return els;
	},
	postJSON: function(url, params, callback) {
		var http = new XMLHttpRequest();

		http.open("POST", url, true);

		// Send the proper header information along with the request
		http.setRequestHeader("Content-type", "application/json");

		http.onreadystatechange = function() {
			if (http.readyState == 4) {
				callback(http.status, JSON.parse(http.responseText));
			}
		}
		http.send(JSON.stringify(params));
	},
};

String.prototype.leftTrim = function() {
	return this.replace(/^\s+/,"");
};
