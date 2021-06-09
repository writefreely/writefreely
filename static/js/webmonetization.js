/*
 * Copyright © 2020-2021 A Bunch Tell LLC.
 *
 * This file is part of WriteFreely.
 *
 * WriteFreely is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License, included
 * in the LICENSE file in this source code package.
 */

let unlockingSplitContent = false;
let unlockedSplitContent = false;
let pendingSplitContent = false;

function showWMPaywall($content, $split) {
    let $readmoreSell = document.createElement('div')
    $readmoreSell.id = 'readmore-sell';
    $content.insertAdjacentElement('beforeend', $readmoreSell);
    $readmoreSell.appendChild($split);
    $readmoreSell.insertAdjacentHTML("beforeend", '\n\n<p class="font sans">For <strong>$5 per month</strong>, you can read this and other great writing across our site and other websites that support Web Monetization.</p>')
    $readmoreSell.insertAdjacentHTML("beforeend", '\n\n<p class="font sans"><a href="https://coil.com/signup?ref=writefreely" class="btn cta" target="coil">Get started</a> <a href="https://coil.com/?ref=writefreely" class="btn cta secondary">Learn more</a></p>')
}

function initMonetization() {
    let $content = document.querySelector('.e-content')
    let $post = document.getElementById('post-body')
    let $split = $post.querySelector('.split')
    if (document.monetization === undefined || $split == null) {
        if ($split) {
            showWMPaywall($content, $split)
        }
        return
    }

    document.monetization.addEventListener('monetizationstop', function(event) {
        if (pendingSplitContent) {
            // We've seen the 'pending' activity, so we can assume things will work
            document.monetization.removeEventListener('monetizationstop', progressHandler)
            return
        }

        // We're getting 'stop' without ever starting, so display the paywall.
        showWMPaywall($content, $split)
    });

    document.monetization.addEventListener('monetizationpending', function (event) {
        pendingSplitContent = true
    })

    let progressHandler = function(event) {
        if (unlockedSplitContent) {
            document.monetization.removeEventListener('monetizationprogress', progressHandler)
            return
        }
        if (!unlockingSplitContent && !unlockedSplitContent) {
            unlockingSplitContent = true
            getSplitContent(event.detail.receipt, function (status, data) {
                unlockingSplitContent = false
                if (status == 200) {
                    $split.textContent = "Your subscriber perks start here."
                    $split.insertAdjacentHTML("afterend", "\n\n"+data.data.html_body)
                } else {
                    $split.textContent = "Something went wrong while unlocking subscriber content."
                }
                unlockedSplitContent = true
            })
        }
    }

    function getSplitContent(receipt, callback) {
        let params = "receipt="+encodeURIComponent(receipt)

        let http = new XMLHttpRequest();
        http.open("POST", "/api/collections/" + window.collAlias + "/posts/" + window.postSlug + "/splitcontent", true);

        // Send the proper header information along with the request
        http.setRequestHeader("Content-type", "application/x-www-form-urlencoded");

        http.onreadystatechange = function () {
            if (http.readyState == 4) {
                callback(http.status, JSON.parse(http.responseText));
            }
        }
        http.send(params);
    }

    document.monetization.addEventListener('monetizationstart', function() {
        if (!unlockedSplitContent) {
            $split.textContent = "Unlocking subscriber content..."
        }
        document.monetization.removeEventListener('monetizationstart', progressHandler)
    });
    document.monetization.addEventListener('monetizationprogress', progressHandler);
}