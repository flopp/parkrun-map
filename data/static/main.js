var on_load = function(f) {
    if (document.body === null) {
        document.addEventListener('DOMContentLoaded', () => {f()}, false);
    } else {
        f();
    }
}

const updateTracks = function(map, parkruns) {
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

    // TABLE
    if (document.getElementById("parkruns-table") && window.jQuery && window.jQuery.fn.dataTable) {
        // Add custom type detection and sorting
        window.jQuery.fn.dataTable.ext.type.order['de_date-pre'] = function (d) {
            if (!d || d === '-') return 0;
            // Remove HTML tags if present
            d = d.replace(/<[^>]*>/g, '').trim();
            var parts = d.split('.');
            if (parts.length !== 3) return 0;
            // Return as YYYYMMDD integer for sorting
            return parseInt(parts[2] + parts[1].padStart(2, '0') + parts[0].padStart(2, '0'), 10);
        };
        // Custom sorting for 'Letzter Lauf' (number + suffix or '-')
        window.jQuery.fn.dataTable.ext.type.order['laufnum-pre'] = function (d) {
            if (!d || d === '-') return 0;
            // Remove HTML tags if present
            d = d.replace(/<[^>]*>/g, '').trim();
            // Extract the number before the first space or parenthesis
            var match = d.match(/^(\d+)/);
            if (match) return parseInt(match[1], 10);
            return 0;
        };
        // Custom sorting for 'Rang' (numbers and '-')
        window.jQuery.fn.dataTable.ext.type.order['rangnum-pre'] = function (d) {
            if (!d || d === '-') return Number.POSITIVE_INFINITY;
            // Remove HTML tags if present
            d = d.replace(/<[^>]*>/g, '').trim();
            var n = parseInt(d, 10);
            return isNaN(n) ? Number.POSITIVE_INFINITY : n;
        };
        var dt = window.jQuery('#parkruns-table').DataTable({
            responsive: false,
            ordering: true,
            columnDefs: [
                { targets: 2, type: 'de_date' }, // 'Seit'
                { targets: 3, type: 'laufnum' }, // 'Letzter Lauf'
                { targets: 5, type: 'rangnum' }  // 'Rang'
            ],
            paging: false,
            searching: false,
            language: {}
        });
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