import { useEffect, useState } from "react";
import { fetchClient } from "../../shared/api/client";
import { StatusBadge } from "../../shared/components/DataStates";
import type { FloodEvent } from "../../shared/types/flood";

export function AIEvidenceReview() {
  const [events, setEvents] = useState<FloodEvent[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadAIEvidence();
  }, []);

  const loadAIEvidence = async () => {
    setLoading(true);
    try {
      // In a real app we'd have a specific endpoint or filter by source_type='AI' and verification_status='UNVERIFIED'
      // Mocking fetch for Phase 10 structure
      const data = await fetchClient("/api/v1/flood-events?source=AI_VISION_V1");
      setEvents(data.items || []);
    } catch (e) {
      console.error(e);
    } finally {
      setLoading(false);
    }
  };

  const verifyEvidence = async (id: string, accept: boolean) => {
    try {
      await fetchClient(`/api/v1/admin/flood-events/${id}/${accept ? 'verify' : 'reject'}`, {
        method: "POST",
      });
      setEvents(events.filter(e => e.id !== id));
    } catch (e) {
      console.error(e);
      alert("Failed to review evidence");
    }
  };

  if (loading) return <div className="p-8 text-center text-gray-500">กำลังโหลด...</div>;

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
                onClick={() => verifyEvidence(event.id, true)}
                className="px-4 py-2 bg-green-600 text-white rounded hover:bg-green-700 transition-colors"
              >
                ยืนยัน (อนุมัติ)
              </button>
              <button
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
