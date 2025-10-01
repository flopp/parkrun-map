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
    const greenIcon = load_marker("green");
    const greyIcon = load_marker("grey");

    parkruns.forEach((parkrun, index, array) => {
        if (parkrun.active) {
            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: blueIcon});
            //const marker = L.circleMarker([parkrun.lat, parkrun.lon], {color: "darkblue", fillColor: "blue", fillOpacity: 1, radius: 8});
            marker
                .addTo(map)
                .bindPopup(`<a href="${parkrun.id}.html"><b>${parkrun.name}</b></a><br>${parkrun.location}`);
        } else if (parkrun.planned) {
            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: greenIcon});
            marker
                .addTo(map)
                .bindPopup(`<a href="${parkrun.id}.html"><b>${parkrun.name}</b></a> <span class="tag is-success is-light">geplant</span><br>${parkrun.location}`);
        } else {
            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: greyIcon});
            marker
                .addTo(map)
                .bindPopup(`<a href="${parkrun.id}.html"><b>${parkrun.name}</b></a> <span class="tag is-danger is-light">archiviert</span><br>${parkrun.location}`);    
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


const loadParkrunMap = function (divId) {
    const div = document.getElementById(divId);
    const parkrunId = div.dataset.id;
    
    let parkrun = null;
    parkruns.forEach((p) => {
        if (p.id === parkrunId) {
            parkrun = p;
        }
    });

    if (parkrun == null) {
        div.style.display = "none";
    } else {
        const map = L.map(divId, {preferCanvas: true});
        L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
            attribution: '&copy; <a target="_blank" href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
        }).addTo(map);

        const latLng = L.latLng(parkrun.lat, parkrun.lon);
        const bounds = L.latLngBounds(latLng, latLng);
        if (parkrun.active) {
            const blueIcon = load_marker("");
            const marker = L.marker(latLng, {icon: blueIcon});
            marker.addTo(map);    
        } else {
            const redIcon = load_marker("red");
            const marker = L.marker(latLng, {icon: redIcon});
            marker.addTo(map);    
        }

        parkrun.tracks.forEach(latlngs => {
            bounds.extend(L.latLngBounds(latlngs));
            L.polyline(latlngs, {color: 'red'}).addTo(map);

        });
        map.fitBounds(bounds);
    }
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
    } else if (document.querySelector("#parkrun-map") !== null) {
        mapId = "parkrun-map";
        loadParkrunMap(mapId);
    }
};

on_load(main);