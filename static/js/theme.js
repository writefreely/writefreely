(function () {
  var STORAGE_KEY = "padTheme";
  var LIGHT = "light";
  var DARK = "dark";

  function preferredTheme() {
    return window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? DARK : LIGHT;
  }

  function storedTheme() {
    try {
      return localStorage.getItem(STORAGE_KEY);
    } catch (e) {
      return null;
    }
  }

  function currentTheme() {
    return storedTheme() || preferredTheme();
  }

  function setTheme(theme) {
    var next = theme === DARK ? DARK : LIGHT;
    document.documentElement.classList.remove(LIGHT, DARK);
    document.documentElement.classList.add(next);
    if (document.body) {
      document.body.classList.remove(LIGHT, DARK);
      document.body.classList.add(next);
    }
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch (e) {
      // ignore storage errors
    }
    updateThemeIcons(next);
  }

  function toggleTheme() {
    setTheme(document.documentElement.classList.contains(DARK) ? LIGHT : DARK);
  }

  function updateThemeIcons(theme) {
    var btns = document.querySelectorAll('#tools a img, [data-theme-toggle] img');
    for (var i = 0; i < btns.length; i++) {
      var img = btns[i];
      if (!img.src) {
        continue;
      }
      if (/ic_brightness(_dark)?@2x\.png$/.test(img.src)) {
        img.src = theme === LIGHT
          ? img.src.replace(/ic_brightness(_dark)?@2x\.png$/, 'ic_brightness_dark@2x.png')
          : img.src.replace(/ic_brightness(_dark)?@2x\.png$/, 'ic_brightness@2x.png');
        continue;
      }
      if (theme === LIGHT) {
        if (!/_dark@2x\.png$/.test(img.src)) {
          img.src = img.src.replace(/@2x\.png$/, '_dark@2x.png');
        }
      } else {
        img.src = img.src.replace(/_dark@2x\.png$/, '@2x.png');
      }
    }
  }

  function bindToggle(el) {
    if (!el || el.dataset.themeBound === "true") {
      return;
    }
    el.dataset.themeBound = "true";
    el.addEventListener("click", function (e) {
      e.preventDefault();
      toggleTheme();
    });
  }

  function init() {
    setTheme(currentTheme());
    bindToggle(document.getElementById("toggle-theme"));
    var more = document.querySelectorAll("[data-theme-toggle]");
    for (var i = 0; i < more.length; i++) {
      bindToggle(more[i]);
    }
  }

  window.toggleTheme = toggleTheme;
  window.setTheme = setTheme;

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
