var on_load = function(f) {
    if (document.body === null) {
        document.addEventListener('DOMContentLoaded', () => {f()}, false);
    } else {
        f();
    }
}

const updateTracks = function(map, parkruns) {
    // zoomed out => hide all tracks
    if (map.getZoom() <= 10) {
        parkruns.forEach((parkrun, index, array) => {
            if (parkrun.polylines_visible) {
                array[index].polylines_visible = false;
                parkrun.polylines.forEach(p => {
                    p.removeFrom(map);
                });
            }
        });
        return;
    }


    // show tracks within bounds
    const bounds = map.getBounds();
    parkruns.forEach((parkrun, index, array) => {
        if (bounds.contains([parkrun.lat, parkrun.lon])) {
            if (!parkrun.polylines_visible) {
                array[index].polylines_visible = true;
                // newly create leaflet polyline
                if (parkrun.polylines === null) {
                    array[index].polylines = [];
                    parkrun.tracks.forEach(latlngs => {
                        array[index].polylines.push(L.polyline(latlngs, {color: 'red'}));
                    });
                }
                parkrun.polylines.forEach(p => {
                    p.addTo(map);
                });
            }
        } else if (parkrun.polylines_visible) {
            array[index].polylines_visible = false;
            parkrun.polylines.forEach(p => {
                p.removeFrom(map);
            });
        }
    });
};

const loadMap = function (id) {
    const germany = [
        [50.913868, 5.603027],
        [55.329144, 8.041992],
        [50.999929, 15.227051],
        [47.034162, 10.217285]
    ];
    var map = L.map(id, {preferCanvas: true}).fitBounds(germany);
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a target="_blank" href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map);

    const blueIcon = load_marker("");
    const redIcon = load_marker("red");

    parkruns.forEach((parkrun, index, array) => {
        let latest = "Letzte Austragung: keine";
        if (parkrun.latest !== null) {
            latest = `Letzte Austragung:<br><a target="_blank" href="${parkrun.url}/results/${parkrun.latest.index}">#${parkrun.latest.index}</a> am ${parkrun.latest.date} mit ${parkrun.latest.runners} Teilnehmern`;
        }
        if (parkrun.active) {
            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: blueIcon});
            //const marker = L.circleMarker([parkrun.lat, parkrun.lon], {color: "darkblue", fillColor: "blue", fillOpacity: 1, radius: 8});
            marker
                .addTo(map)
                .bindPopup(`<a target="_blank" href="${parkrun.url}"><b>${parkrun.name}</b></a><br>${parkrun.location}<br><br>${latest}`);
        } else {
            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: redIcon});
            //const marker = L.circleMarker([parkrun.lat, parkrun.lon], {color: "darkred", fillColor: "red", fillOpacity: 1, radius: 8});
            marker
                .addTo(map)
                .bindPopup(`<a target="_blank" href="${parkrun.url}"><b>${parkrun.name}</b></a> <span class="tag is-danger is-light">archiviert</span><br>${parkrun.location}<br><br>${latest}`);
        }
        array[index].polylines = null;
        array[index].polylines_visible = false;
    });

    map.on('zoomend', function() {
        updateTracks(map, parkruns);
    });

    map.on('moveend', function() {
        updateTracks(map, parkruns);
    });
    updateTracks(map, parkruns);
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