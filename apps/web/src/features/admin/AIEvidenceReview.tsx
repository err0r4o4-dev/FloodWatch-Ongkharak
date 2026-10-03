import { useEffect, useRef, useState } from "react";
import { confirmAction } from "../../shared/alerts";
import { apiConfig } from "../../shared/config/api";
import { StatusBadge } from "../../shared/components/DataStates";
import type { FloodEvent } from "../../shared/types/flood";

export function AIEvidenceReview() {
  const [events, setEvents] = useState<FloodEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [reviewing, setReviewing] = useState(false);
  const reviewPending = useRef(false);

  useEffect(() => {
    loadAIEvidence();
  }, []);

  const loadAIEvidence = async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const response = await fetch(`${apiConfig.baseUrl}/flood-events?source=AI_VISION_V1`);
      if (response.ok) {
         const data = await response.json();
         setEvents(data.items || []);
      } else throw new Error("Evidence unavailable");
    } catch (e) {
      setLoadError("ไม่สามารถโหลดหลักฐาน AI ได้ กรุณาลองใหม่อีกครั้ง");
    } finally {
      setLoading(false);
    }
  };

  const verifyEvidence = async (id: string, accept: boolean) => {
    if (reviewPending.current) return;
    reviewPending.current = true;
    setReviewing(true);
    try {
      await confirmAction({
        title: accept ? "อนุมัติหลักฐาน AI หรือไม่?" : "ปฏิเสธหลักฐาน AI หรือไม่?",
        text: "โปรดตรวจสอบหลักฐานก่อนบันทึกผล การดำเนินการนี้ไม่เปลี่ยนแหล่งที่มาของข้อมูล AI",
        confirmText: accept ? "อนุมัติหลักฐาน" : "ปฏิเสธหลักฐาน",
        danger: !accept,
        action: async () => {
          const response = await fetch(`${apiConfig.baseUrl}/admin/flood-events/${id}/${accept ? 'verify' : 'reject'}`, { method: "POST" });
          if (!response.ok) throw new Error("Failed to review evidence");
          setEvents((current) => current.filter((event) => event.id !== id));
        },
        successTitle: accept ? "อนุมัติหลักฐานแล้ว" : "ปฏิเสธหลักฐานแล้ว",
        errorText: "ไม่สามารถบันทึกผลการตรวจสอบหลักฐาน AI ได้ กรุณาลองใหม่อีกครั้ง"
      });
    } finally {
      reviewPending.current = false;
      setReviewing(false);
    }
  };

  if (loading) return <div className="p-8 text-center text-gray-500">กำลังโหลด...</div>;
  if (loadError) return <div className="p-8" role="alert"><p>{loadError}</p><button type="button" className="action-button" onClick={() => void loadAIEvidence()}>ลองอีกครั้ง</button></div>;

  if (events.length === 0) return <div className="p-8 text-center text-gray-500">ไม่มีหลักฐาน AI ที่รอการตรวจสอบ</div>;

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold">ตรวจสอบหลักฐานจาก AI</h2>
      <div className="grid gap-4">
        {events.map(event => (
          <div key={event.id} className="bg-white dark:bg-gray-800 p-4 rounded shadow border border-gray-200 dark:border-gray-700 flex gap-4">
            <div className="flex-1">
              <div className="flex justify-between items-start">
                <h3 className="font-bold text-lg">{event.title}</h3>
                <StatusBadge status={event.verificationStatus} />
              </div>
              <p className="text-sm text-gray-600 dark:text-gray-400 mt-1">{event.description}</p>

              <div className="mt-4 grid grid-cols-2 gap-2 text-sm">
                <div>
                  <span className="text-gray-500">ระดับความรุนแรง:</span>
                  <span className="ml-2 font-medium">{event.severity}</span>
                </div>
                <div>
                  <span className="text-gray-500">ความมั่นใจ:</span>
                  <span className="ml-2 font-medium">{event.confidence ? `${(event.confidence * 100).toFixed(0)}%` : 'N/A'}</span>
                </div>
              </div>
            </div>

            <div className="flex flex-col gap-2 justify-center border-l pl-4 border-gray-200 dark:border-gray-700">
              <button
                disabled={reviewing}
                onClick={() => verifyEvidence(event.id, true)}
                className="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition-colors"
              >
                ยืนยัน (อนุมัติ)
              </button>
              <button
                disabled={reviewing}
                onClick={() => verifyEvidence(event.id, false)}
                className="px-4 py-2 bg-red-600 text-white rounded hover:bg-red-700 transition-colors"
              >
                ปฏิเสธ (ลบ)
              </button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
