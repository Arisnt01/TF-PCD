import { useRef, useEffect, useState } from "react";
import "./Carrusel.css";

export default function Carrusel({
  title,
  data = [],
  loading = false,
  visibleCount = 5, //cantidad de tarjetas visibles
}) {
  const viewportRef = useRef(null);
  const trackRef = useRef(null);
  const [scrollAmount, setScrollAmount] = useState(0);

  useEffect(() => {
    const calc = () => {
      const track = trackRef.current;
      const viewport = viewportRef.current;
      if (!track || !viewport) return;

      const firstCard = track.querySelector(".card");
      const gap = 16;
      if (!firstCard) return;

      const cardWidth = firstCard.getBoundingClientRect().width;
      const amount = Math.round((cardWidth * visibleCount) + gap * (visibleCount - 1));
      viewport.style.width = `${amount}px`;
      setScrollAmount(amount);
    };

    calc();
    window.addEventListener("resize", calc);
    return () => window.removeEventListener("resize", calc);
  }, [data, visibleCount]);

  const scrollLeft = () => {
    if (!trackRef.current) return;
    trackRef.current.scrollBy({ left: -scrollAmount, behavior: "smooth" });
  };

  const scrollRight = () => {
    if (!trackRef.current) return;
    trackRef.current.scrollBy({ left: scrollAmount, behavior: "smooth" });
  };

  return (
    <div className="carrusel-container full">
      <h2 className="carrusel-title">{title}</h2>

      {loading ? (
        <p>Cargando...</p>
      ) : data.length === 0 ? (
        <p>No hay datos disponibles.</p>
      ) : (
        <div className="carrusel-wrapper center">
          <button className="arrow left" onClick={scrollLeft} aria-label="Anterior">◀</button>

          <div className="viewport" ref={viewportRef}>
            <div className="track" ref={trackRef}>
              {data.map((item) => (
                <div className="card" key={item.movie_id}>
                  {item.poster ? (
                    <img src={item.poster} alt={item.title} />
                  ) : (
                    <div className="placeholder" />
                  )}
                  <p className="movie-title">{item.title}</p>
                  {item.rating !== undefined ? (
                    <p className="movie-meta">⭐ {item.rating}</p>
                  ) : item.predicted_score !== undefined ? (
                    <p className="movie-meta">Predicción: ⭐ {item.predicted_score.toFixed(2)}</p>
                  ) : null}
                </div>
              ))}
            </div>
          </div>

          <button className="arrow right" onClick={scrollRight} aria-label="Siguiente">▶</button>
        </div>
      )}
    </div>
  );
}