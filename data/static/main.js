var on_load = function(f) {
    if (document.body === null) {
        document.addEventListener('DOMContentLoaded', () => {f()}, false);
    } else {
        f();
    }
}

const trackPanes = {
    archived: 'parkrun-tracks-archived-pane',
    planned: 'parkrun-tracks-planned-pane',
    active: 'parkrun-tracks-active-pane'
};

const ensureTrackPanes = function(map) {
    if (!map.getPane(trackPanes.archived)) {
        map.createPane(trackPanes.archived);
    }
    if (!map.getPane(trackPanes.planned)) {
        map.createPane(trackPanes.planned);
    }
    if (!map.getPane(trackPanes.active)) {
        map.createPane(trackPanes.active);
    }
    map.getPane(trackPanes.archived).style.zIndex = 350;
    map.getPane(trackPanes.planned).style.zIndex = 360;
    map.getPane(trackPanes.active).style.zIndex = 370;
};

const getTrackStyle = function(parkrun) {
    if (parkrun.active) {
        return {color: 'red', pane: trackPanes.active};
    }
    if (parkrun.planned) {
        return {color: 'red', pane: trackPanes.planned};
    }
    return {color: 'grey', pane: trackPanes.archived};
};

const createToiletMarker = function(toilet, pane) {
    console.log("toilet at", toilet.lat, toilet.lon);
    const marker = L.marker([toilet.lat, toilet.lon], {
        icon: L.divIcon({
            className: 'toilet-map-marker',
            html: '<span aria-hidden="true">WC</span>',
            iconSize: [36, 36],
            iconAnchor: [18, 18],
            popupAnchor: [0, -18]
        }),
        title: toilet.name || 'Toilette',
        pane: pane
    });
    marker.bindTooltip(toilet.name || 'Toilette');

    const popup = document.createElement('div');
    const name = document.createElement('strong');
    name.textContent = toilet.name || 'Toilette';
    popup.append(name);
    marker.bindPopup(popup);
    return marker;
};

const updateToilets = function(map, toiletMarkers) {
    const bounds = map.getBounds();
    const shouldShow = map.getZoom() >= 15;
    toiletMarkers.forEach(toilet => {
        const visible = shouldShow && bounds.contains(toilet.latlng);
        if (visible === toilet.visible) {
            return;
        }
        toilet.visible = visible;
        if (visible) {
            toilet.marker.addTo(map);
            console.log("showing toilet at", toilet.latlng);
        } else {
            toilet.marker.removeFrom(map);
            console.log("hiding toilet at", toilet.latlng);
        }
    });
};

const updateTracks = function(map, parkruns, toiletMarkers) {
    ensureTrackPanes(map);
    updateToilets(map, toiletMarkers);
    // store lat,lon,zoom in location.hash
    const center = map.getCenter();
    const zoom = map.getZoom();
    location.hash = `${center.lat.toFixed(5)}/${center.lng.toFixed(5)}/${zoom}`;

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
        const courseVisible = parkrun.trackBounds && bounds.intersects(parkrun.trackBounds);
        if (bounds.contains([parkrun.lat, parkrun.lon]) || courseVisible) {
            if (!parkrun.polylines_visible) {
                const style = getTrackStyle(parkrun);
                array[index].polylines_visible = true;
                // newly create leaflet polyline
                if (parkrun.polylines === null) {
                    array[index].polylines = [];
                    parkrun.tracks.forEach(latlngs => {
                        array[index].polylines.push(L.polyline(latlngs, style));
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

const fixLeafletButtons = (div) => {
    div.querySelectorAll('[role="button"]').forEach((btn) => {
        btn.removeAttribute('role');
    });
};

const loadMap = function (id, hash) {
    // parse hash for lat, lon, zoom
    var lat, lon, zoom = -1;
    if (hash) {
        const parts = hash.substring(1).split("/");
        if (parts.length === 3) {
            lat = parseFloat(parts[0]);
            lon = parseFloat(parts[1]);
            zoom = parseInt(parts[2], 10);
            if (isNaN(lat) || isNaN(lon) || isNaN(zoom)) {
                lat = null;
                lon = null;
                zoom = -1;
            }
        }
    }
    var map = L.map(id, {preferCanvas: true});
    if (lat !== null && lon !== null && zoom !== -1) {
        map.setView([lat, lon], zoom);
    } else {
        const germany = [
            [50.913868, 5.603027],
            [55.329144, 8.041992],
            [50.999929, 15.227051],
            [47.034162, 10.217285]
        ];
        map.fitBounds(germany);
    }
    L.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a target="_blank" href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map);

    const blueIcon = load_marker("");
    const redIcon = load_marker("red");
    const greenIcon = load_marker("green");
    const greyIcon = load_marker("grey");

    const toiletMarkers = [];
    const seenToilets = new Set();
    let minAttendace = 0;
    let maxAttendance = 0;
    if (!map.getPane('toiletPane')) {
        map.createPane('toiletPane');
        map.getPane('toiletPane').style.zIndex = 350;
    }

    parkruns.forEach((parkrun, index, array) => {
        (parkrun.toilets || []).forEach(toilet => {
            const key = `${toilet.lat},${toilet.lon}`;
            if (!seenToilets.has(key)) {
                seenToilets.add(key);
                toiletMarkers.push({
                    latlng: L.latLng(toilet.lat, toilet.lon),
                    marker: createToiletMarker(toilet, 'toiletPane'),
                    visible: false
                });
            }
        });

        if (parkrun.active) {
            if (parkrun.latest) {
                if (parkrun.latest.runners > maxAttendance) {
                    maxAttendance = parkrun.latest.runners;
                }
                if (parkrun.latest.runners < minAttendace || minAttendace === 0) {
                    minAttendace = parkrun.latest.runners;
                }
            }
        }
    });

    parkruns.forEach((parkrun, index, array) => {
        if (parkrun.active) {
            if (parkrun.latest) {
                // add scaled circle based on attendance
                const attendance = parkrun.latest.runners;
                const radius = 100 + (attendance - minAttendace) / (maxAttendance - minAttendace) * 5000;
                const circle = L.circle([parkrun.lat, parkrun.lon], {
                    color: 'blue',
                    fillColor: 'blue',
                    opacity: 0.2,
                    fillOpacity: 0.1,
                    radius: radius
                });
                circle.addTo(map);
            }

            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: blueIcon, zIndexOffset: 2000});
            //const marker = L.circleMarker([parkrun.lat, parkrun.lon], {color: "darkblue", fillColor: "blue", fillOpacity: 1, radius: 8});
            marker
                .addTo(map)
                .bindPopup(`<a href="${parkrun.id}.html"><b>${parkrun.name}</b></a><br>${parkrun.location}`);
        } else if (parkrun.planned) {
            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: greenIcon, zIndexOffset: 1000});
            marker
                .addTo(map)
                .bindPopup(`<a href="${parkrun.id}.html"><b>${parkrun.name}</b></a> <span class="tag is-success is-light">geplant</span><br>${parkrun.location}`);
        } else if (parkrun.temporarily_closed) {
            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: greyIcon, zIndexOffset: 0});
            marker
                .addTo(map)
                .bindPopup(`<a href="${parkrun.id}.html"><b>${parkrun.name}</b></a> <span class="tag is-danger is-light">temporär geschlossen</span><br>${parkrun.location}`);    
        
        } else {
            const marker = L.marker([parkrun.lat, parkrun.lon], {icon: greyIcon, zIndexOffset: 0});
            marker
                .addTo(map)
                .bindPopup(`<a href="${parkrun.id}.html"><b>${parkrun.name}</b></a> <span class="tag is-danger is-light">archiviert</span><br>${parkrun.location}`);    
        }
        array[index].polylines = null;
        array[index].polylines_visible = false;
        const trackCoordinates = parkrun.tracks.flat();
        array[index].trackBounds = trackCoordinates.length > 0 ? L.latLngBounds(trackCoordinates) : null;
    });

    map.on('zoomend', function() {
        updateTracks(map, parkruns, toiletMarkers);
    });
    map.on('moveend', function() {
        updateTracks(map, parkruns, toiletMarkers);
    });
    updateTracks(map, parkruns, toiletMarkers);
    fixLeafletButtons(document.getElementById(id));
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
        // no preferCanvas here: canvas-rendered tracks cover the whole map and would
        // block pointer events to the toilet markers in the pane underneath
        const map = L.map(divId);
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
        // toilet markers use a pane below the default overlayPane (z-index 400) so the track stays on top
        if (!map.getPane('toiletPane')) {
            map.createPane('toiletPane');
            map.getPane('toiletPane').style.zIndex = 350;
        }
        (parkrun.toilets || []).forEach(toilet => {
            const toiletLatLng = L.latLng(toilet.lat, toilet.lon);
            bounds.extend(toiletLatLng);
            createToiletMarker(toilet, 'toiletPane').addTo(map);
        });
        map.fitBounds(bounds);

        fixLeafletButtons(div);
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
    const originalHash = location.hash;

    // MAPS
    var mapId = "";
    if (document.getElementById("map") !== null) {
        mapId = "map";
        loadMap(mapId, originalHash);
    } else if (document.getElementById("parkrun-map") !== null) {
        mapId = "parkrun-map";
        loadParkrunMap(mapId);
    }

    // UMAMI
    document.querySelectorAll("a[target=_blank]").forEach((a) => {
        if (a.getAttribute("data-umami-event") === null) {
            a.setAttribute('data-umami-event', 'outbound-link-click');
        }
        a.setAttribute('data-umami-event-url', a.href);
    });
    if (originalHash === '#disable-umami') {
        console.log("Disabling Umami in this browser.");
        localStorage.setItem('umami.disabled', 'true');
        alert('Umami is now DISABLED in this browser.');
    }
    if (originalHash === '#enable-umami') {
        console.log("Enabling Umami in this browser.");
        localStorage.removeItem('umami.disabled');
        alert('Umami is now ENABLED in this browser.');
    }
};

on_load(main);