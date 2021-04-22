/*
 * Copyright © 2016-2021 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

function showModal(id) {
    document.getElementById('overlay').style.display = 'block';
    document.getElementById('modal-'+id).style.display = 'block';
}

var closeModals = function(e) {
    e.preventDefault();
    document.getElementById('overlay').style.display = 'none';
    var modals = document.querySelectorAll('.modal');
    for (var i=0; i<modals.length; i++) {
        modals[i].style.display = 'none';
    }
};
H.getEl('overlay').on('click', closeModals);