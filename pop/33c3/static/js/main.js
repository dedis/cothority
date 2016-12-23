//var log = console.log
var log = alert;

$(document).ready(function(){
    setupMediaDevices();
    
    var config = "";
    var privateKey = "";
   
    refreshEntryTable();
   
    if (getCookie("33c3-Cookie") == "") {
        scanPhase();
    } else {
        log("cookie: " + getCookie("33c3-Cookie"));
    }

    setInterval(function() {    
        refreshEntryTable();
    },3000);

});


function scanPhase() {
    var qr = new QCodeDecoder();

    if (!(qr.isCanvasSupported() && qr.hasGetUserMedia())) {
      alert('Your browser doesn\'t match the required specs.');
      throw new Error('Canvas and getUserMedia are required');
    }   
    alert("Please first scan the PoP-Party QR Code (front cover)");
    decodeQR(qr).then(function(resultConfig) {
        if (resultConfig.indexOf("sha256:") == -1) {
            return new Promise(function(resolve,reject) {
                reject("config text is not correct" + resultConfig);
            });
        }
        config = resultConfig.slice("hash:".length);;
        alert("QR Code decoded correctly. Now the private key (inside)");
        qr.stop()
        qr = new QCodeDecoder();
        return decodeQR(qr);
    }).then(function(resultPrivate) {
        if (resultPrivate.indexOf("ed25519priv:") == -1) {
            return new Promise(function(resolve,reject) {
                reject("private key is not correct" + resultPrivate);
            });
        }
        privateKey = resultPrivate.slice("ed25519priv:".length);
        log("Private key decoded correctly.\nProceed to signing message and get cookie from server");
        qr.stop()
        // hide the video
        $("video").hide();
        return get("siginfo");
    }).then(function(info) {
        return login(info,privateKey);
    }).then(function(tag) {
        alert("Well done, you are now logged in!");
    }).catch(function(err){
        log("Error: " + JSON.stringify(err));
    });
    /*    var info = "";*/
    //var config = "";
    //var privateKey = "ZHxWRZO1h391k0Uqv/PyjUfO3sx5lMLGhXk5iRaAdQM=";
    //get("siginfo").then(function(getInfo) {
        //info = getInfo;
        //log("siginfo returned correctly");
        //return login(info,privateKey);
    //}).then(function(tag) {
        //log("tag = " + ascii_to_hexa(tag));
    //}).catch(function(err){
        //log("catch error: " + JSON.stringify(err));
    //});
    //get("entries").then(function(data) {
        //fillEntryTable(data);
    /*});*/

}

function refreshEntryTable() {
    get("entries").then(function(data) {
        fillEntryTable(data);
    });
}

function getCookie(cname) {
    var name = cname + "=";
    var ca = document.cookie.split(';');
    for(var i = 0; i <ca.length; i++) {
        var c = ca[i];
        while (c.charAt(0) == ' ') {
            c = c.substring(1);
        }
        if (c.indexOf(name) == 0) {
            return c.substring(name.length, c.length);
        }
    }
    return "";
}

// make a get call to the URI and returns a Promise
function get(uri) {
    return new Promise(function(resolv,reject) {
        $.get(uri,function(data) {
            resolv(data);
        });
    });
}

// Fill out the entries in a supa nice html table
function fillEntryTable(data) {
    var json = $.parseJSON(data)  
    $('tbody#entry-body').html('');
    $.each(json,function(i,item) {
        var voted = "no";
        if (item.Voted) {
           voted = "yes"; 
        }
        // empty the div
        var $tr = $('<tr>').append(
                //$('<td>').text(item.Id),
                $('<td>').text(item.Name),
                $('<td>').text(item.Room),
                $('<td>').text(item.Date),
                $('<td>').text(item.Duration),
                $('<td>',{id:"entry-up-"+item.Id}).text(item.Up),
                $('<td>',{id:"entry-down-"+item.Id}).text(item.Down),
                $('<td>').html('<button type="submit" id="vote-up-' + item.Id + '">Up</button>'),
                $('<td>').html('<button type="submit" id="vote-down-' + item.Id + '">Down</button>')).appendTo('tbody#entry-body');

        var upButton = $('#vote-up-'+item.Id);
        var upVote = $('#entry-up-'+item.Id);
        var downButton = $('#vote-down-'+item.Id);
        var downVote = $('#entry-down-'+item.Id);
        upButton.click(function() {
            $.ajax("vote",  { data: { index: item.Id, vote: true }, type: "POST",
                error: function(err) {
                    alert("You can not vote: " + JSON.stringify(err))
                },
                success: function(data) {
                    updateVotes(upButton,downButton);
                }
            });
        });

        downButton.click(function() {
            $.ajax("vote",  { data: { index: item.Id, vote: false }, type: "POST",
                error: function(err) {
                    alert("You can not vote: " + JSON.stringify(err))
                },
                success: function(data) {
                    updateVotes(downButton,upButton);
                }
            });
        })
    });
};

function updateVotes(selectedVote,otherVote) {
    v1 = parseInt(selectedVote.text());
    v2 = parseInt(otherVote.text());
    v1 = v1 + 1;
    if (v1 < 0) {
        v1 = 0;
    } 
    v2 = v2 - 1;
    if (v2 < 0) {
        v2 = 0;
    }
    selectedVote.text(v1);
    otherVote.text(v2);
}

function plusone(val) {
    return val+1;
}

function minusone(val) {
    return val -1;
}

function decodeQR(qr) {
    var video = document.querySelector("video");
    var reset = document.querySelector("#reset");
    var stop = document.querySelector("#stop");

    return new Promise(function (accept,reject) {
         var found = false;
         function resultHandler (err, result) {
          if (err || found){
            // drop it silently
            return;
          }  
          found = true;
          accept(result);
        };
        // prepare a canvas element that will receive
        // the image to decode, sets the callback for
        // the result and then prepares the
        // videoElement to send its source to the
        // decoder.
        qr.decodeFromCamera(video, resultHandler);
        // attach some event handlers to reset and
        // stop whenever we want.
        /*reset.onclick = function () {*/
          //qr.decodeFromCamera(video, resultHandler);
        //};
        //stop.onclick = function () {
          //qr.stop();
        /*};*/
    });
};

function login(loginInfo,privateKey) {
    return new Promise(function(resolve,reject) {
        ret = sig.Sign(privateKey,loginInfo);
        sigLogin = ret[0];
        err = ret[1];
        if (err != "") 
            log("error signature:" + JSON.stringify(err));

        $.ajax("login",  { data: sigLogin, type: "POST",
            error: function(err) {
               reject(err); 
            },
            success: function(data) {
                resolve(data);
            }
        });

        /*$.post("login",sigLogin,function (data,statusResp) {*/
            //if (!statusResp)
                //reject("login post " + data);
            //resolve(data);
        //}).error(function(err) {
            //reject(err);
        /*});*/
    });
}

// unify all calls for media devices API
function setupMediaDevices() {
    // Older browsers might not implement mediaDevices at all, so we set an empty object first
    if (navigator.mediaDevices === undefined) {
      navigator.mediaDevices = {};
      alert("no mediaDevices :(");
    }

    // Some browsers partially implement mediaDevices. We can't just assign an object
    // with getUserMedia as it would overwrite existing properties.
    // Here, we will just add the getUserMedia property if it's missing.
    if (navigator.mediaDevices.getUserMedia === undefined) {
        alert("no getUserMedia :(");
      navigator.mediaDevices.getUserMedia = function(constraints) {

        // First get ahold of the legacy getUserMedia, if present
        var getUserMedia = (navigator.getUserMedia ||
          navigator.webkitGetUserMedia ||
          navigator.mozGetUserMedia);

        // Some browsers just don't implement it - return a rejected promise with an error
        // to keep a consistent interface
        if (!getUserMedia) {
            alert("getUserMedia impossible :(");
          return Promise.reject(new Error('getUserMedia is not implemented in this browser'));
        }

        // Otherwise, wrap the call to the old navigator.getUserMedia with a Promise
        return new Promise(function(resolve, reject) {
          getUserMedia.call(navigator, constraints, resolve, reject);
        });
      }
    }
}

function ascii_to_hexa(str)  
  {  
    var arr1 = [];  
    for (var n = 0, l = str.length; n < l; n ++)   
     {  
        var hex = Number(str.charCodeAt(n)).toString(16);  
        arr1.push(hex);  
     }  
    return arr1.join('');  
   } 
