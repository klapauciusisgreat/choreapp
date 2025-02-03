// Function to handle chore completion
async function handleChoreCompletion(checkbox) {
    const choreItem = checkbox.closest('li');

    // Get the current computed style for opacity
    const computedStyle = window.getComputedStyle(choreItem);
    const currentOpacity = computedStyle.opacity;


    if (checkbox.checked) {
        choreItem.style.transition = "opacity 0.5s ease-out";
        choreItem.style.opacity = 0.5; // Fade out
        document.getElementById('choreCompleteSound').play();
    } else {
        choreItem.style.transition = "opacity 0.5s ease-in";
        choreItem.style.opacity = 1; // Fade in
    }

    // Prevent default form submission
    event.preventDefault();

    // Get the form data
    const formData = new FormData(checkbox.form);

    try {
        // Send an asynchronous POST request to the server
        const response = await fetch(checkbox.form.action, {
            method: 'POST',
            body: formData
        });

        if (response.ok) {
            // Handle a successful response (e.g., update the UI)

            // Fetch and update the chores list
            fetchAndUpdateChores();

	    // Fetch and update the points data
            fetchAndUpdatePoints();

        } else {
            // Handle errors
            console.error("Error updating chore:", response.statusText);
        }
    } catch (error) {
        console.error("Error updating chore:", error);
    }
}

// Function to handle chore claiming
async function handleChoreClaim(button) {
    const choreItem = button.closest('li');
    choreItem.style.transition = "background-color 0.5s ease-out";
    choreItem.style.backgroundColor = "#e0ffe0"; // Light green

    // Prevent default form submission
    event.preventDefault();

    // Get the form data
    const formData = new FormData(button.form);

    try {
        // Send an asynchronous POST request to the server
        const response = await fetch(button.form.action, {
            method: 'POST',
            body: formData
        });

        if (response.ok) {
            // Handle a successful response (e.g., update the UI)
	    document.getElementById('choreClaimSound').play();


            // Fetch and update the chores list
            fetchAndUpdateChores();

        } else {
            // Handle errors
            console.error("Error claiming chore:", response.statusText);
        }
    } catch (error) {
        console.error("Error claiming chore:", error);
    }
}

// Function to fetch chores data from the server and update the UI
async function fetchAndUpdateChores() {
    try {
        const response = await fetch('/chores');
        if (response.ok) {
            const chores = await response.json();
            updateChoresList(chores);
        } else {
            console.error("Error fetching chores:", response.statusText);
        }
    } catch (error) {
        console.error("Error fetching chores:", error);
    }
}

// Function to fetch points data from the server and update the graphs
async function fetchAndUpdatePoints() {
    try {
        const response = await fetch('/points'); // New endpoint for points
        if (response.ok) {
            const pointsData = await response.json();
            updatePointsGraphs(pointsData);
        } else {
            console.error("Error fetching points data:", response.statusText);
        }
    } catch (error) {
        console.error("Error fetching points:", error);
    }
}

// Function to update the points graphs with new data
function updatePointsGraphs(pointsData) {
    createBarChart(pointsData.dailyData, 'daily-chart', 'steelblue');
    createBarChart(pointsData.weeklyData, 'weekly-chart', 'orange');
}

function updateChoresList(chores) {
    // Get the current user's ID
    const currentUserId = window.currentUserId;

    // Update the "Chores for Today" list
    const choresForTodayList = document.querySelector('#my-chores');
    choresForTodayList.innerHTML = '';

    const choresForToday = chores.filter(chore => chore.IsAssigned);

    if (choresForToday.length > 0) {
        choresForToday.forEach(chore => {
            const listItem = document.createElement('li');
            listItem.innerHTML = `
                <form id="form-${chore.ID}" action="/chore/update" method="POST">
                    <input type="hidden" name="chore_id" value="${chore.ID}">
                    <input type="hidden" name="completed" value="${!chore.Completed}">
                    <label>
                        <input type="checkbox" name="completed_checkbox" ${chore.Completed ? 'checked' : ''} onchange="handleChoreCompletion(this)">
                        ${chore.Name} (${chore.Points} points)
                    </label>
                </form>
            `;
            choresForTodayList.appendChild(listItem);
        });
    } else {
        choresForTodayList.innerHTML = '<li>No chores assigned to you today!</li>';
    }

    // Update the "Chores Available to Claim" list
    const choresToClaimList = document.querySelector('#claim-chores');
    choresToClaimList.innerHTML = '';

    const choresToClaim = chores.filter(chore => chore.IsClaimable);

    if (choresToClaim.length > 0) {
        choresToClaim.forEach(chore => {
            const listItem = document.createElement('li');
            listItem.innerHTML = `
                <form id="claim-form-${chore.ID}" action="/chore/claim" method="POST">
                    <input type="hidden" name="chore_id" value="${chore.ID}">
                    ${chore.Name} (${chore.Points} points)
                    <button type="button" onclick="handleChoreClaim(this)">Claim</button>
                </form>
            `;
            choresToClaimList.appendChild(listItem);
        });
    } else {
        choresToClaimList.innerHTML = '<li>No chores available to claim!</li>';
    }
}

async function initializeChores() {
    try {
        const response = await fetch('/chores');
        if (response.ok) {
            const chores = await response.json();
            updateChoresList(chores);
        } else {
            console.error("Error fetching initial chores:", response.statusText);
        }

        // Also fetch and update the points data initially
        fetchAndUpdatePoints();
    } catch (error) {
        console.error("Error fetching initial chores:", error);
    }
}

function createBarChart(data, svgId, barColor) {
    const svg = document.getElementById(svgId);
    const svgWidth = svg.width.baseVal.value;
    const svgHeight = svg.height.baseVal.value;
    const barPadding = 5;
    const barWidth = (svgWidth / data.length) - barPadding;
    const isWeekly = svgId === 'weekly-chart';

    const maxDataValue = Math.max(...data);
    const yScale = maxDataValue > 0 ? (svgHeight - 20) / maxDataValue : 0; // Reserve space for labels

    svg.innerHTML = ''; // Clear existing bars and labels

    const today = new Date(); // Get today's date

    data.forEach((value, index) => {
        const barHeight = value === 0 ? 1 : value * yScale * 0.9; // Minimum height of 1px for 0 values
        const y = svgHeight - barHeight - 20; // Adjust for label space
        const x = index * (barWidth + barPadding);

        // Create bar
        const bar = document.createElementNS("http://www.w3.org/2000/svg", "rect");
        bar.setAttribute("x", x);
        bar.setAttribute("y", y);
        bar.setAttribute("width", barWidth);
        bar.setAttribute("height", barHeight);
        bar.setAttribute("fill", barColor);
        svg.appendChild(bar);

        // Add value label inside the bar
        const valueLabel = document.createElementNS("http://www.w3.org/2000/svg", "text");
        valueLabel.setAttribute("x", x + barWidth / 2);
        valueLabel.setAttribute("y", y + barHeight / 2);
        valueLabel.setAttribute("text-anchor", "middle");
        valueLabel.setAttribute("dominant-baseline", "central");
        valueLabel.setAttribute("fill", "white");
        valueLabel.textContent = value;
        svg.appendChild(valueLabel);

        // Create date label
        const date = new Date(today);
        if (isWeekly) {
            // For weekly chart, subtract 7 days for each index
            date.setDate(today.getDate() - (data.length - 1 - index) * 7);
        } else {
            // For daily chart, subtract 1 day for each index
            date.setDate(today.getDate() - (data.length - 1 - index));
        }

        const day = String(date.getDate()).padStart(2, '0');
        const month = String(date.getMonth() + 1).padStart(2, '0');

        const dateLabel = document.createElementNS("http://www.w3.org/2000/svg", "text");
        dateLabel.setAttribute("x", x + barWidth / 2);
        dateLabel.setAttribute("y", svgHeight - 5);
        dateLabel.setAttribute("text-anchor", "middle");
        dateLabel.setAttribute("font-size", "10px");
        dateLabel.textContent = `${day}/${month}`;
        svg.appendChild(dateLabel);
    });
}

// Call fetchAndUpdateChores when the page loads
window.addEventListener('load', fetchAndUpdateChores);

// Call initializeChores when the page loads
window.addEventListener('load', initializeChores);

