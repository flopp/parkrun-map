var on_load = function(f) {
    if (document.body === null) {
        document.addEventListener('DOMContentLoaded', () => {f()}, false);
    } else {
        f();
    }
}

const loadMap = function (id) {
    const germany = [
        [50.913868, 5.603027],
        [55.329144, 8.041992],
        [50.999929, 15.227051],
        [47.034162, 10.217285]
    ];
    var map = L.map(id).fitBounds(germany);
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map);

    const blueIcon = load_marker("");
    parkruns.forEach(parkrun => {
        let latest = "Letzte Austragung: keine";
        if (parkrun.latest !== null) {
            latest = `Letzte Austragung:<br><a href="${parkrun.latest.url}">#${parkrun.latest.index}</a> am ${parkrun.latest.date} mit ${parkrun.latest.runners} Teilnehmern`;
        }
        const marker = L.marker([parkrun.lat, parkrun.lon], {icon: blueIcon}).addTo(map)
            .bindPopup(`<a href="${parkrun.url}"><b>${parkrun.name}</b></a><br>${parkrun.location}<br><br>${latest}`);
        console.log("TRACKS:", parkrun.tracks);
        parkrun.tracks.forEach(track => {
            console.log("TRACK:", track);
            const latlngs = [];
            track.forEach(c => {
                console.log("COORDS:", c);
                //latlngs.push(L.LatLng(c.Lat, c.Lon));
                latlngs.push([c.Lat, c.Lon]);
            });
            console.log("L-TRACK:", latlngs);
            //var polyline = L.polyline(latlngs, {color: 'red'}).addTo(map);
        });
    });
};

var load_marker = function (color) {
    let url = "/images/marker-icon.png";
    let url2x = "/images/marker-icon-2x.png";
    if (color !== "") {
        url = "/images/marker-" + color + "-icon.png";
        url2x = "/images/marker-" + color + "-icon-2x.png";
    }
    let options = {
        iconAnchor: [12, 41],
        iconRetinaUrl: url2x,
        iconSize: [25, 41],
        iconUrl: url,
        popupAnchor: [1, -34],
        shadowSize: [41, 41],
        shadowUrl: "/images/marker-shadow.png",
        tooltipAnchor: [16, -28],
    };
    return L.icon(options);
}

var main = () => {
    var mapId = "";
    if (document.querySelector("#map") !== null) {
        mapId = "map";
        loadMap(mapId);
    }
};

on_load(main);