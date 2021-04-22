var menuItems = document.querySelectorAll('li.has-submenu');
var menuTimer;
function closeMenu($menu) {
    $menu.querySelector('a').setAttribute('aria-expanded', "false");
    $menu.className = "has-submenu";
}
Array.prototype.forEach.call(menuItems, function(el, i){
    el.addEventListener("mouseover", function(event){
        let $menu = document.querySelectorAll(".has-submenu.open");
        if ($menu.length > 0) {
            closeMenu($menu[0]);
        }
        this.className = "has-submenu open";
        this.querySelector('a').setAttribute('aria-expanded', "true");
        clearTimeout(menuTimer);
    });
    el.addEventListener("mouseout", function(event){
        menuTimer = setTimeout(function(event){
            let $menu = document.querySelector(".has-submenu.open");
            closeMenu($menu);
        }, 500);
    });
    el.querySelector('a').addEventListener("click",  function(event){
        if (this.parentNode.className == "has-submenu") {
            this.parentNode.className = "has-submenu open";
            this.setAttribute('aria-expanded', "true");
        } else {
            this.parentNode.className = "has-submenu";
            this.setAttribute('aria-expanded', "false");
        }
        event.preventDefault();
        return false;
    });
});