import confetti from "canvas-confetti";

const colors = ["#9b6bc9", "#a855c9", "#c9a8e8", "#c47ab8", "#f2cd8c", "#dce0f4"];

function at(rect?: DOMRect) {
    if (!rect) return { x: 0.5, y: 0.45 };
    return {
        x: (rect.left + rect.width / 2) / window.innerWidth,
        y: (rect.top + rect.height / 2) / window.innerHeight
    };
}

/** the small burst one finished item earns */
export function cheer(rect?: DOMRect) {
    const origin = at(rect);
    confetti({
        particleCount: 60,
        spread: 58,
        startVelocity: 30,
        scalar: 0.82,
        ticks: 130,
        gravity: 1.1,
        origin,
        colors,
        disableForReducedMotion: true
    });
    confetti({
        particleCount: 24,
        spread: 110,
        startVelocity: 16,
        scalar: 0.6,
        ticks: 110,
        origin,
        colors,
        disableForReducedMotion: true
    });
}

/** clearing a whole day, or the whole week, is worth more than one item */
export function ovation(scale: "day" | "week") {
    const count = scale === "week" ? 150 : 90;
    for (const x of [0.2, 0.8]) {
        confetti({
            particleCount: count / 2,
            spread: 70,
            startVelocity: scale === "week" ? 52 : 42,
            scalar: 1,
            ticks: 200,
            origin: { x, y: 0.72 },
            angle: x < 0.5 ? 60 : 120,
            colors,
            disableForReducedMotion: true
        });
    }
}
